package main

import (
	"bufio"
	"fmt"
	"os"
	"stackpath-demonstration-app/pkg/stackpath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
)

// Program configuration
const (
	APIClientID      = "set me"
	APIClientSecret  = "set me"
	StackSlug        = "set me"
	DomainName       = "set me"
	ProjectSubDomain = "set me"
)

// These entities are built as the app is deployed to StackPath.
var (
	client         *stackpath.Client
	stack          *stackpath.Stack
	domain         *stackpath.Domain
	workload       *stackpath.Workload
	site           *stackpath.Site
	deliveryDomain string
)

func main() {
	// There are various pauses in the process with prompts to press [Enter] to
	// continue. Read that from STDIN when necessary.
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(`
StackPath Platform Demo
=======================

Welcome to our demo! 

This program provisions an Edge Compute container workload with a diagnostic web 
application in multiple cities with auto-scaling, puts the app behind 
StackPath's CDN and WAF, provisions a DNS entry for it, adds demonstration WAF 
rules, then sets up an auto-renewing SSL certificate for the final app.

After the app is provisioned, this program will monitor the WAF for security 
events and monitor Edge Compute logs for web app requests and new instance start 
up and tear down. 

The only things that exist prior to this are the project's stack and a 
registered domain name with an empty zone provisioned on our DNS infrastructure. 
This program was written from scratch and uses the StackPath REST API for all 
interaction with StackPath.

This is a live demo. Fingers crossed, everyone!

Press [Enter] to continue.`)
	_, _ = reader.ReadString('\n')

	// Editor's note: Normally I'd write more idiomatic code here with proper
	// variable scoping, parameter and error handling, and no display side
	// effects. These happy-path functions handle all of that internally. They
	// show the steps needed to provision a full application stack without
	// having to get too far into coding bits, making a demo of the process a
	// little easier to read.

	fmt.Println(`Checking requirements
---------------------`)
	authenticateToStackPath()
	findStack()
	findDomainOnStack()

	fmt.Println(`Requirements met!
Press [Enter] to continue.`)
	_, _ = reader.ReadString('\n')

	fmt.Println(`Deploying the application
-------------------------`)
	provisionComputeWorkload()
	provisionSite()
	waitForComputeWorkload()
	findDeliveryDomain()
	setDNSCNAMERecord()
	provisionSSLCertificate()
	createWAFRules()

	fmt.Printf("Success! The project is available at https://%s.%s\n", ProjectSubDomain, DomainName)
	fmt.Println("Press [Enter] to begin monitoring the application")
	fmt.Println("Press [q] then [Enter] to end the program")
	_, _ = reader.ReadString('\n')

	// Monitor the apps in functions that run concurrently echo'ing to STDOUT.
	go displayWAFRequests()
	go displayInstanceLogs()
	go func() {
		for {
			select {}
		}
	}()

	_, _ = reader.ReadString('q')
	fmt.Println("Done")
	fmt.Println()
}

// authenticateToStackPath populates the `client` variable with an authenticated
// StackPath API bearer token.
func authenticateToStackPath() {
	var err error
	s, t := startSpinner("Authenticating to StackPath")

	client, err = stackpath.NewClient(APIClientID, APIClientSecret)
	if err != nil {
		donef("Error Authenticating to StackPath: %s", err)
	}

	stopSpinner(s, t, "Done", false)
}

// findStack checks if the `StackSlug` stack exists and populates `stack` with
// the stack if so.
func findStack() {
	var err error
	s, t := startSpinner("Finding the project stack")

	stack, err = client.FindStackBySlug(StackSlug)
	if err != nil {
		donef("Error locating stack: %s", err)
	}
	if stack == nil {
		stopSpinner(s, t, "Not found", false)
		donef("Stack \"%s\" was not found", StackSlug)
	}

	stopSpinner(s, t, fmt.Sprintf("Done: found stack \"%s\" (slug: %s)", stack.Name, stack.Slug), false)
}

// findDomainOnStack looks for the `DomainName` domain on the `stack` stack and
// populates `domain` if so.
func findDomainOnStack() {
	var err error
	s, t := startSpinner(fmt.Sprintf("Locating the \"%s\" DNS zone", DomainName))

	domain, err = client.FindDomainByName(stack, DomainName)
	if err != nil {
		donef("Error locating DNS Zone: %s", err)
	}
	if domain == nil {
		stopSpinner(s, t, "Not found", false)
		donef("DNS zone \"%s\" was not found", DomainName)
	}

	stopSpinner(s, t, fmt.Sprintf("Done: found DNS zone \"%s\" (ID: %s)", domain.Name, domain.ID), false)
}

// provisionComputeWorkload creates a new Edge Compute workload on the StackPath
// platform and populates `workload` the new workload object.
func provisionComputeWorkload() {
	var err error
	s, t := startSpinner("Creating compute workload")

	workload, err = client.CreateWorkload(stack)
	if err != nil {
		donef("Error creating compute workload: %s", err)
	}

	stopSpinner(
		s,
		t,
		fmt.Sprintf("Done: workload \"%s\" created, anycast IP: %s", workload.Name, workload.AnycastIP),
		true,
	)
}

// provisionSite creates CDN and WAF service using the workload's anycast IP as
// the origin and populates `site` with the resulting site object.
func provisionSite() {
	var err error
	s, t := startSpinner("Creating CDN and WAF service in front of the Edge Compute origin")

	site, err = client.CreateSiteDelivery(stack, workload.AnycastIP, fmt.Sprintf("%s.%s", ProjectSubDomain, DomainName))
	if err != nil {
		donef("Error creating CDN and WAF service: %s", err)
	}

	stopSpinner(s, t, fmt.Sprintf("Done: site \"%s\" created", site.ID), true)
}

// waitForComputeWorkload tracks the instances in `workload` and echos when
// their state changes. It uses a spinner as a loading screen while waiting on
// the first instance. This doesn't use but emulates startSpinner()'s and
// stopSpinner()'s behavior because there's custom echo'ing to the console while
// the workload starts.
func waitForComputeWorkload() {
	fmt.Println("Waiting for all containers to start before continuing")
	t := time.Now()
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "| Waiting for the first instance to start "
	s.Start()

	// instanceStatus is a mapping of instance name -> status
	instanceStatus := make(map[string]string, 0)

	// Poll for instance status once per second. Display the spinner until the
	// first instance starts. After that report instance status changes to the
	// console. Quit the ticker after at least 3 instances are running, a fair
	// assumption that all workload instances started.
	for {
		instances, err := client.GetInstances(stack, workload)
		if err != nil {
			donef("Error querying instance status: %s", err)
		}

		if len(instances) == 0 {
			continue
		}

		s.Stop()

		allInstancesRunning := true
		for i, instance := range instances {
			_, found := instanceStatus[instance.Name]

			if !found || instanceStatus[instance.Name] != instance.Phase {
				if i == 0 {
					fmt.Println()
				}

				fmt.Printf("| Instance \"%s\" is %s\n", instance.Name, strings.ToLower(instance.Phase))
				instanceStatus[instance.Name] = instance.Phase
			}

			if instance.Phase != "RUNNING" {
				allInstancesRunning = false
			}
		}
		if allInstancesRunning && len(instances) >= 3 {
			break
		}

		time.Sleep(time.Second)
	}

	fmt.Println("| Done")
	fmt.Printf("└ Took %v\n\n", time.Now().Sub(t))
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

// findDeliveryDomain looks for `site`'s delivery domain, also called an edge
// address, and populates it in `deliveryDomain`. The delivery domain is used as
// a DNS CNAME target for the project's subdomain.
func findDeliveryDomain() {
	var err error
	s, t := startSpinner("Locating the site's delivery domain")

	deliveryDomain, err = client.FindSiteDeliveryDomain(stack, site)
	if err != nil {
		donef("Error locating the site's delivery domain: %s", err)
	}

	stopSpinner(s, t, fmt.Sprintf("Done: found the delivery domain \"%s\"", deliveryDomain), true)
}

// setDNSCNAMERecods creates the project's DNS CNAME record, using to the site's
// delivery domain as the target.
func setDNSCNAMERecord() {
	s, t := startSpinner(fmt.Sprintf("Creating the project DNS record: \"%s.%s\"", ProjectSubDomain, DomainName))

	err := client.SetDNSCNAME(stack, domain, ProjectSubDomain, deliveryDomain)
	if err != nil {
		donef("Error creating project DNS CNAME: %s", err)
	}

	stopSpinner(s, t, "Done", true)
}

// provisionSSLCertificate requests an SSL certificate on `site`.
func provisionSSLCertificate() {
	s, t := startSpinner("Creating an SSL certificate")

	err := client.RequestFreeSSLCert(stack, site)
	if err != nil {
		donef("Error creating an SSL certificate: %s", err)
	}

	stopSpinner(s, t, "Done", true)
}

// createWAFRules creates a demo block rule on `site`.
func createWAFRules() {
	s, t := startSpinner("Creating custom WAF rules")

	err := client.CreateDemoWAFRules(stack, site)
	if err != nil {
		donef("Error creating custom WAF rule: %s", err)
	}

	stopSpinner(s, t, "Done", true)
}

// displayWAFRequests polls the WAF for a request log once a second and sends
// formatted logs to STDOUT.
func displayWAFRequests() {
	mostRecentRequestTime := time.Now().Add(time.Hour * 24 * -30)

	for {
		requests, err := client.GetWAFRequests(stack, site, mostRecentRequestTime)
		if err != nil {
			donef("Error getting WAF requests: %s", err)
		}

		for i, request := range requests {
			fullRuleName := ""
			if request.RuleName != "" {
				fullRuleName = ": " + request.RuleName
			}

			fmt.Printf(
				"[WAF %s%s] %s %s %s - %s (%s) - %s\n",
				request.Action,
				fullRuleName,
				request.RequestTime,
				request.Method,
				request.Path,
				request.ClientIP,
				request.Country,
				request.UserAgent,
			)

			if i == len(requests)-1 {
				mostRecentRequestTime = request.RequestTime.Add(time.Second)
			}
		}

		time.Sleep(time.Second)
	}
}

// displayInstanceLogs polls the workload for instances once a second and loads
// the instance's console logs, echo'ing every log line to STDOUT.
func displayInstanceLogs() {
	mostRecentRequestTime := time.Now().Add(time.Hour * 24 * -30)
	instanceStatus := make(map[string]string, 0)
	i := 0

	for {
		instances, err := client.GetInstances(stack, workload)
		if err != nil {
			donef("Error querying workload instances: %s", err)
		}

		for _, instance := range instances {
			// Look for status changes
			//
			// On first run populate the instance status map, so we can watch
			// for changes later.
			if i == 0 {
				instanceStatus[instance.Name] = instance.Phase
			} else {
				// Look for the instance in the status map. If it's not there
				// then it's a new instance. Otherwise, if the phase is
				// different, then the instance is in a new status.
				phase, found := instanceStatus[instance.Name]
				if !found {
					fmt.Printf("[New instance %s] instance is %s\n", instance.Name, strings.ToLower(instance.Phase))
					instanceStatus[instance.Name] = instance.Phase
				} else if phase != instance.Phase {
					fmt.Printf("[%s] instance is now %s\n", instance.Name, strings.ToLower(instance.Phase))
					instanceStatus[instance.Name] = instance.Phase
				}
			}

			// Get and echo the instance's logs.
			logs, err := client.GetInstanceLogs(stack, workload, &instance, mostRecentRequestTime)
			if err != nil {
				donef("Error querying %s instance logs: %s", instance.Name, err)
			}

			scanner := bufio.NewScanner(strings.NewReader(logs))

			for scanner.Scan() {
				fmt.Printf("[%s] %s\n", instance.Name, scanner.Text())
			}
		}

		// Check for instances that went away. They'd show up in the map but not
		// in the retrieved instance list.
		if i != 0 {
			newInstanceStatus := make(map[string]string, 0)

			for checkName, _ := range instanceStatus {
				found := false

				for _, instance := range instances {
					if checkName == instance.Name {
						found = true
						newInstanceStatus[checkName] = instance.Phase
					}
				}

				if !found {
					fmt.Printf("[%s] instance went away\n", checkName)
				}
			}

			instanceStatus = newInstanceStatus
		}

		i++
		mostRecentRequestTime = time.Now()
		time.Sleep(time.Second)
	}
}

// startSpinner wraps spinner.New() with a common charset and duration, sets a
// spinner prefix, and starts the spinner. It returns the spinner and a
// time.Time object so stopSpinner() can stop the spinner and calculate a time
// duration later.
func startSpinner(prefix string) (*spinner.Spinner, time.Time) {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = prefix + " "
	s.Start()

	return s, time.Now()
}

// stopSpinner stops a *spinner.Spinner created by startSpinner() and echos a
// message and time duration.
func stopSpinner(s *spinner.Spinner, t time.Time, message string, pauseAtTheEnd bool) {
	s.Stop()
	fmt.Printf("\n| %s\n", message)
	fmt.Printf("└ Took %s\n\n", time.Now().Sub(t))

	if pauseAtTheEnd {
		fmt.Println("Press [Enter] to continue.")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
}

// donef is a wrapper to exit the program with the exit code 1 and a message
func donef(format string, a ...interface{}) {
	fmt.Printf(format+"\n", a...)
	fmt.Println("Done")
	fmt.Println()
	os.Exit(1)
}
