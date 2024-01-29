package copper

import "strings"

type Option func(c *config)

type config struct {
	basePath                  string
	checkInternalServerErrors bool
	checkRequestBody          bool
}

func getConfig(opts ...Option) config {
	c := &config{}
	for _, opt := range opts {
		opt(c)
	}

	return *c
}

// WithBasePath is a functional Option for setting the base path used when correlating the specification to the API
// calls being recorded.
func WithBasePath(path string) Option {
	return func(c *config) {
		c.basePath = "/" + strings.Trim(path, "/")
	}
}

// WithInternalServerErrors is a functional Option for also validating server responses. These are skipped by default
// since a server should not ideally have internal server errors, and even if they are not part of a specification, they
// considered a possible response from an API.
func WithInternalServerErrors() Option {
	return func(c *config) {
		c.checkInternalServerErrors = true
	}
}

// WithRequestBodyValidation is a functional Option for checking request bodies as they are sent. Doing validation of
// the body by default might conflict with checking error cases (400 responses specifically), so it does not happen by
// default. Enabling checking will produce an error for each request that contains a body that is not in accordance with
// the specification for that endpoint.
func WithRequestBodyValidation() Option {
	return func(c *config) {
		c.checkRequestBody = true
	}
}
