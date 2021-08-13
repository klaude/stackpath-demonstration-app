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

// Site models a StackPath CDN delivery site.
type Site struct {
	ID string `json:"id"`
}

// WAFRequest models an individual request captured by the StackPath WAF.
// Requests have key aspects of the client's HTTP request against the site and
// the action the WAF took.
type WAFRequest struct {
	ID          string    `json:"id"`
	Action      string    `json:"action"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	ClientIP    string    `json:"clientIp"`
	Country     string    `json:"country"`
	UserAgent   string    `json:"userAgent"`
	RuleName    string    `json:"ruleName"`
	RequestTime time.Time `json:"requestTime"`
}

// CreateSiteDelivery creates a delivery site on the StackPath CDN with WAF
// service enabled.
//
// See: https://stackpath.dev/reference/sites#createsite-1
func (c *Client) CreateSiteDelivery(stack *Stack, originIP, domainName string) (*Site, error) {
	reqBody := bytes.NewBuffer([]byte(`{
  "domain": "` + domainName + `",
  "origin": {
    "path": "/",
    "hostname": "` + originIP + `",
    "port": 80
  },
  "features": ["CDN", "WAF"],
  "configuration": {
    "originPullProtocol": {
      "protocol": "http"
    }
  }
}`))
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(baseURL+"/delivery/v1/stacks/%s/sites", stack.Slug),
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

	newSite := struct {
		Site Site `json:"site"`
	}{}
	err = json.Unmarshal(body, &newSite)
	if err != nil {
		return nil, err
	}

	return &newSite.Site, nil
}

// FindSiteDeliveryDomain retrieves a site's delivery domain, a hostname at
// StackPath that fronts a site's CDN service. An empty string return value
// means no delivery domains were found.
//
// See: https://stackpath.dev/reference/delivery-domains#getsitedeliverydomains2
func (c *Client) FindSiteDeliveryDomain(stack *Stack, site *Site) (string, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(baseURL+"/delivery/v1/stacks/%s/sites/%s/delivery_domains", stack.Slug, site.ID),
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

	results := struct {
		Results []struct {
			Domain string `json:"domain"`
		} `json:"results"`
	}{}
	err = json.Unmarshal(body, &results)
	if err != nil {
		return "", err
	}

	// A site may have more than one delivery domain. We need the one on the
	// stackpathcdn.com domain.
	for _, result := range results.Results {
		if strings.HasSuffix(result.Domain, ".stackpathcdn.com") {
			return result.Domain, nil
		}
	}

	return "", nil
}

// RequestFreeSSLCert provisions an auto-renewing free SSL certificate on the
// given site. Verification is done automatically over DNS.
//
// See: https://stackpath.dev/reference/ssl-1#requestcertificate
func (c *Client) RequestFreeSSLCert(stack *Stack, site *Site) error {
	reqBody := bytes.NewBuffer([]byte(`{
  "verificationMethod": "DNS"
}`))
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(baseURL+"/cdn/v1/stacks/%s/sites/%s/certificates/request", stack.Slug, site.ID),
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
