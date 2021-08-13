package stackpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Workload models a StackPath Edge Compute workload.
type Workload struct {
	ID        string
	Slug      string
	Name      string
	AnycastIP string
}

// Instance models a StackPath Edge Compute workload instance. Instances are the
// VMs and containers that are running in a workload.
type Instance struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Phase             string `json:"phase"`
	IPAddress         string `json:"ipAddress"`
	ExternalIPAddress string `json:"externalIpAddress"`
}

// CreateWorkload creates an Edge Compute workload suitable for demonstration
// purposes.
//
// The workload will have the following characteristics:
// * The name "My compute origin"
// * An anycast IP
// * Instances based on the kennethreitz/httpbin:latest container
// * An overridden command to send httpbin's access logs to STDOUT
// * A single network interface per instance
// * 1 CPU core and 2 GiB of memory per instance
// * Port TCP/80 exposed from the container with public Internet access to it
// * Instances in Frankfurt DE, Amsterdam NL, and Dallas, TX, US
// * Autoscaling from one instance in each POP to two when an instance reaches
//   50% CPU load.
//
// See: https://stackpath.dev/reference/workloads#createworkload
func (c *Client) CreateWorkload(stack *Stack) (*Workload, error) {
	reqBody := bytes.NewBuffer([]byte(`{
  "workload": {
    "name": "My compute origin",
    "metadata": {
      "version": "1",
      "annotations": {
        "anycast.platform.stackpath.net": "true"
      }
    },
    "spec": {
      "networkInterfaces": [
        {
          "network": "default"
        }
      ],
      "containers": {
        "my-app": {
          "image": "kennethreitz/httpbin:latest",
          "command": ["gunicorn", "--access-logfile", "-", "-b", "0.0.0.0:80", "httpbin:app", "-k", "gevent", "--worker-tmp-dir", "/dev/shm"],
          "ports": {
            "http": {
              "port": 80,
              "enableImplicitNetworkPolicy": true
            }
          },
          "resources": {
            "requests": {
              "cpu": "1",
              "memory": "2Gi"
            }
          }
        }
      }
    },
    "targets": {
      "north-america": {
        "spec": {
          "deploymentScope": "cityCode",
          "deployments": {
            "minReplicas": 1,
            "maxReplicas": 2,
            "selectors": [
              {
                "key": "cityCode",
                "operator": "in",
                "values": [
                  "DFW"
                ]
              }
            ],
            "scaleSettings": {
              "metrics": [
                {
                  "metric": "cpu",
                  "averageUtilization": "50"
                }
              ]
            }
          }
        }
      },
      "europe": {
        "spec": {
          "deploymentScope": "cityCode",
          "deployments": {
            "minReplicas": 1,
            "maxReplicas": 2,
            "selectors": [
              {
                "key": "cityCode",
                "operator": "in",
                "values": [
                  "FRA", "AMS"
                ]
              }
            ],
            "scaleSettings": {
              "metrics": [
                {
                  "metric": "cpu",
                  "averageUtilization": "50"
                }
              ]
            }
          }
        }
      }
    }
  }
}`))
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(baseURL+"/workload/v1/stacks/%s/workloads", stack.Slug),
		reqBody,
	)
	if err != nil {
		return nil, err
	}

	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = res.Body.Close()
	if err != nil {
		return nil, err
	}

	newWorkload := struct {
		Workload struct {
			ID       string `json:"id"`
			Slug     string `json:"slug"`
			Name     string `json:"name"`
			Metadata struct {
				Annotations struct {
					AnycastIP string `json:"anycast.platform.stackpath.net/subnets"`
				} `json:"annotations"`
			} `json:"metadata"`
		} `json:"workload"`
	}{}
	err = json.Unmarshal(body, &newWorkload)
	if err != nil {
		return nil, err
	}

	return &Workload{
		ID:        newWorkload.Workload.ID,
		Slug:      newWorkload.Workload.Slug,
		Name:      newWorkload.Workload.Name,
		AnycastIP: strings.Split(newWorkload.Workload.Metadata.Annotations.AnycastIP, "/")[0],
	}, nil
}

// GetInstances gets a compute workload's instances. Instances are the
// containers and VMs that make up the workload.
//
// See: https://stackpath.dev/reference/instances#getworkloadinstances
func (c *Client) GetInstances(stack *Stack, workload *Workload) ([]Instance, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(baseURL+"/workload/v1/stacks/%s/workloads/%s/instances", stack.Slug, workload.Slug),
		nil,
	)
	if err != nil {
		return nil, err
	}

	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = res.Body.Close()
	if err != nil {
		return nil, err
	}

	instanceRes := struct {
		Results []Instance `json:"results"`
	}{}
	err = json.Unmarshal(resBody, &instanceRes)
	if err != nil {
		return nil, err
	}

	return instanceRes.Results, nil
}

// GetInstanceLogs returns an instance's console logs from `since` until now as
// a single string containing line breaks.
//
// See: https://stackpath.dev/reference/instance-logs#getlogs
func (c *Client) GetInstanceLogs(stack *Stack, workload *Workload, instance *Instance, since time.Time) (string, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			baseURL+"/workload/v1/stacks/%s/workloads/%s/instances/%s/logs?timestamps=true&since_time=%s",
			stack.Slug,
			workload.Slug,
			instance.Name,
			since.Format(time.RFC3339),
		),
		nil,
	)
	if err != nil {
		return "", err
	}

	res, err := c.Do(req)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	err = res.Body.Close()
	if err != nil {
		return "", err
	}

	return string(body), nil
}
