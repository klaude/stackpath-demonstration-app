package stackpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// CreateDemoWAFRules creates two demo WAF rules on a site:
// * block requests to /blockme
// * allow requests to /anything
//
// See: https://stackpath.dev/reference/rules#createrule
func (c *Client) CreateDemoWAFRules(stack *Stack, site *Site) error {
	// Make the block rule
	reqBody := bytes.NewBuffer([]byte(`{
  "name": "block access to blockme",
  "description": "A simple path block to demo WAF capabilities",
  "conditions": [
    {
      "url": {
        "url": "/blockme",
        "exactMatch": true
      }
    }
  ],
  "action": "BLOCK",
  "enabled": true
}`))
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(baseURL+"/waf/v1/stacks/%s/sites/%s/rules", stack.Slug, site.ID),
		reqBody,
	)
	if err != nil {
		return err
	}

	_, err = c.Do(req)
	if err != nil {
		return err
	}

	// Make the allow rule
	reqBody = bytes.NewBuffer([]byte(`{
  "name": "allow access to anything",
  "description": "Allow access to a path, regardless of other rules",
  "conditions": [
    {
      "url": {
        "url": "/anything",
        "exactMatch": true
      }
    }
  ],
  "action": "ALLOW",
  "enabled": true
}`))
	req, err = http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(baseURL+"/waf/v1/stacks/%s/sites/%s/rules", stack.Slug, site.ID),
		reqBody,
	)
	if err != nil {
		return err
	}

	_, err = c.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// GetWAFRequests retrieves a site's WAF requests from `since` until now.
//
// See: https://stackpath.dev/reference/requests#getrequests
func (c *Client) GetWAFRequests(stack *Stack, site *Site, since time.Time) ([]WAFRequest, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			baseURL+"/waf/v1/stacks/%s/sites/%s/requests?start_date=%s",
			stack.Slug,
			site.ID,
			since.Format(time.RFC3339),
		),
		nil,
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

	results := struct {
		Results []WAFRequest `json:"results"`
	}{}
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}
