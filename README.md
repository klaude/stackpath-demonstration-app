# A StackPath Platform Demonstration

This is a CLI application that demonstrates provisioning services on the 
StackPath platform then logs various aspects of that app to `STDOUT`. It creates 
CDN and WAF services in front of a container-based Edge Compute workload origin 
along with a DNS CNAME to access the project, a free and auto-renewing SSL 
certificate, and two sample WAF rules.

The Edge Compute origin has instances in Frankfurt DE, Amsterdam NL, and Dallas 
TX USA. Every instance has 1 allocated CPU core and 2 GiB of memory. They 
auto-scale up to two instances in each city if the CPU load goes over 50% in 
that city. It has an anycast IP address to use as a single entrypoint in front 
of the CDN.

Many combinations of applications and services can run on the StackPath 
platform, but for demonstration these containers run the 
[httpbin](https://httpbin.org/) diagnostic application with access logging to 
`STDOUT` in the container, so we can monitor access logs for all running 
containers from the demo. 

In addition to the firewall's standard protection the demo makes a WAF rule that 
blocks access to the path `/blockme` and a rule that allows all requests to 
`/anything` regardless of other rules.

Once everything is started up the demo app dumps new WAF activity, all container 
logs, and container state changes to `STDOUT`. 

This demo communicates with StackPath through the 
[StackPath REST API](https://stackpath.dev/docs/stackpath-api-quick-start). 
Check out the [stackpath](./stackpath) for simple API client and repository 
implementations.

> **Note**: This code is intended for demonstration purposes only. It shows off 
> the capabilities of the StackPath API, but prioritizes happy paths and 
> readability over golang's best practices. Please use this as a reference, but 
> do not base your integration or custom application from this project.

> **Note**: This demo adds live services to your StackPath account but does not 
> remove them. These services will incur charges that you will be invoiced for. 
> Please tear down services after running this demo to avoid large charges.

## Requirements

In order to run this demo you need at least:

* [Go](https://golang.org/). This demo was written in version 1.16, but versions >= 1.13 may also work.
* A StackPath account. [Register a new account](https://control.stackpath.com/register/) at the StackPath portal to get started!
* A StackPath API [client ID and secret pair](https://support.stackpath.com/hc/en-us/articles/360038048431-How-To-Generate-API-Credentials)
* A stack to store demo services on
* A DNS zone provisioned on the stack

## Installation

Check out this repository then run `go mod download` from the project's root 
directory to install the demo's dependencies.

## Configuration

Edit the constants defined near the top of [`main.go`](./main.go) with your API 
client ID, API client secret, the ID or slug of your stack, your project 
domain's FQDN, and the name of the DNS sub-domain you'd the demo to configure. 

## Usage

Run `go run main.go` from the project's root directory to start the demo.

## See Also

* [StackPath](https://stackpath.com/)
* [The StackPath developer portal](https://stackpath.dev/)
