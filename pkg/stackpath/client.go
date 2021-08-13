// Package stackpath is a small repository around StackPath API functionality.
package stackpath

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Client wraps http.Client with a StackPath bearer JWT and has a number of
// repository-like functions to assist in making StackPath API calls.
type Client struct {
	accessToken string
	c           http.Client
}

const (
	userAgent = "forrester-demo-2021"
	baseURL   = "https://gateway.stackpath.com"
)

// NewClient builds a new StackPath API client by authenticating the client ID
// and secret into a bearer token for use in future calls.
//
// See: https://stackpath.dev/reference/authentication#getaccesstoken
func NewClient(apiClientID, apiClientSecret string) (*Client, error) {
	client := &Client{}
	reqBody := bytes.NewBuffer([]byte(`{
  "grant_type": "client_credentials",
  "client_id": "` + apiClientID + `",
  "client_secret": "` + apiClientSecret + `"
}`))
	req, err := http.NewRequest(http.MethodPost, baseURL+"/identity/v1/oauth2/token", reqBody)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
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

	authRes := struct {
		AccessToken string `json:"access_token"`
	}{}
	err = json.Unmarshal(body, &authRes)
	if err != nil {
		return nil, err
	}

	return &Client{
		accessToken: authRes.AccessToken,
		c:           http.Client{},
	}, nil
}

// Do executes a StackPath HTTP request by making a call to the underlying
// http.Client.Do() func. It sets a common user agent request header and treats
//responses whose status codes are greater than or equal to 300 as an error.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	// Set common request headers
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	res, err := c.c.Do(req)
	if err != nil {
		return nil, err
	}

	// Treat all non 2xx responses as errors
	if res.StatusCode >= 300 {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		err = res.Body.Close()
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("%s: %s", res.Status, body)
	}

	return res, nil
}
