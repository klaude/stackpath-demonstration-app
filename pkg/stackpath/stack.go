package stackpath

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

// Stack models a StackPath stack.
type Stack struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// FindStackBySlug searches for a StackPath stack by the given slug. A return value of
// nil means the stack was not found.
//
// See: https://stackpath.dev/reference/stacks#getstacks
func (c *Client) FindStackBySlug(stackSlug string) (*Stack, error) {
	// Search for the stack by slug by passing in a page_request.filter for it.
	req, err := http.NewRequest(
		http.MethodGet,
		baseURL+"/stack/v1/stacks?page_request.filter="+url.QueryEscape("slug=\""+stackSlug+"\""),
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
		Results []Stack `json:"results"`
	}{}
	err = json.Unmarshal(body, &searchRes)
	if err != nil {
		return nil, err
	}

	// If results is empty then the stack slug wasn't found.
	if len(searchRes.Results) == 0 {
		return nil, nil
	}

	return &searchRes.Results[0], nil
}
