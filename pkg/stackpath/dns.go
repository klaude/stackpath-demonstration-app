package stackpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// Domain models a StackPath DNS zone.
type Domain struct {
	ID   string `json:"id"`
	Name string `json:"domain"`
}

// FindDomainByName searches for a DNS zone on a stack with the given name. A
// nil domain result means the domain was not found.
//
// See: https://stackpath.dev/reference/zones#getzones
func (c *Client) FindDomainByName(stack *Stack, domain string) (*Domain, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(
			baseURL+"/dns/v1/stacks/%s/zones?page_request.filter=%s",
			stack.Slug,
			url.QueryEscape("domain=\""+domain+"\""),
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

	searchRes := struct {
		Zones []Domain `json:"zones"`
	}{}
	err = json.Unmarshal(body, &searchRes)
	if err != nil {
		return nil, err
	}

	// If results is empty then the zone wasn't found.
	if len(searchRes.Zones) == 0 {
		return nil, nil
	}

	return &searchRes.Zones[0], nil
}

// SetDNSCNAME creates a DNS CNAME resource record. The record's TTL is 60s.
//
// See: https://stackpath.dev/reference/resource-records#createzonerecord
func (c *Client) SetDNSCNAME(stack *Stack, domain *Domain, record, target string) error {
	reqBody := bytes.NewBuffer([]byte(`{
  "type": "CNAME",
  "name": "` + record + `",
  "data": "` + target + `",
  "ttl": 60
}`))
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf(baseURL+"/dns/v1/stacks/%s/zones/%s/records", stack.Slug, domain.ID),
		reqBody,
	)
	if err != nil {
		return err
	}

	// There's no need to save or interpret the API call response.
	_, err = c.Do(req)
	if err != nil {
		return err
	}

	return nil
}
