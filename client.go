package copper

import (
	"fmt"
	"io"
	"net/http"
)

// ValidatingClient provides an HTTP client, and wraps the main methods, recording any and all paths that are being
// called.
type ValidatingClient struct {
	c *http.Client
	*Verifier
}

type config struct {
	base string
}

type Option func(c *config)

// WithBasePath is a functional Option for setting the base path of the validator.
func WithBasePath(path string) Option {
	return func(c *config) {
		c.base = path
	}
}

// WrapClient takes an HTTP client and io.Reader for the OpenAPI spec. The spec is parsed, and wraps the client so that
// the outbound calls are now recorded when made.
func WrapClient(c *http.Client, spec io.Reader, opts ...Option) (*ValidatingClient, error) {
	s, err := io.ReadAll(spec)
	if err != nil {
		return nil, fmt.Errorf("could not read spec: %w", err)
	}

	conf := &config{}
	for _, opt := range opts {
		opt(conf)
	}

	verifier, err := NewVerifier(s, conf.base)
	if err != nil {
		return nil, fmt.Errorf("could not create verifier: %w", err)
	}

	return &ValidatingClient{
		c:        c,
		Verifier: verifier,
	}, nil
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

// Delete records response for HTTP DELETE requests
func (v ValidatingClient) Delete(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return v.recordResponse(v.c.Do(req))
}

func (v ValidatingClient) recordResponse(resp *http.Response, err error) (*http.Response, error) {
	if err == nil {
		v.Record(resp)
	}
	return resp, err
}
