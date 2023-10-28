package copper

import (
	"io"
	"net/http"
)

// ValidatingClient provides an HTTP client, and wraps the main methods, recording any and all paths that are being
// called.
type ValidatingClient struct {
	c *http.Client
	*Verifier
}

// Do takes any http.Request, sends it to the server it and then records the result.
func (v ValidatingClient) Do(r *http.Request) (*http.Response, error) {
	return v.recordResponse(v.c.Do(r))
}

// Head is a convenience method for recording responses for HTTP HEAD requests
func (v ValidatingClient) Head(url string) (resp *http.Response, err error) {
	return v.recordResponse(v.c.Head(url))
}

// Get is a convenience method for recording responses for HTTP GET requests
func (v ValidatingClient) Get(url string) (resp *http.Response, err error) {
	return v.recordResponse(v.c.Get(url))
}

// Put is a convenience method for recording responses for HTTP PUT requests
func (v ValidatingClient) Put(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest(http.MethodPut, url, body)
	req.Header.Set("Content-Type", contentType)
	if err != nil {
		return nil, err
	}
	return v.recordResponse(v.c.Do(req))
}

// Post is a convenience method for recording responses for HTTP POST requests
func (v ValidatingClient) Post(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	return v.recordResponse(v.c.Post(url, contentType, body))
}

func (v ValidatingClient) recordResponse(resp *http.Response, err error) (*http.Response, error) {
	if err == nil {
		v.Record(resp)
	}
	return resp, err
}

// Delete records response for HTTP DELETE requests
func (v ValidatingClient) Delete(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return v.recordResponse(v.c.Do(req))
}
