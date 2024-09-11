package copper

import "strings"

type Option func(c *config)

type config struct {
	basePath                     string
	checkInternalServerErrors    bool
	checkRequest                 bool
	requestLogger                RequestLogger
	disableFullCoverage          bool
	ignoreUnsupportedBodyFormats bool
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

// WithRequestValidation is a functional Option for checking request parameters and bodies as they are sent. Doing
// validation of the request by default might conflict with checking error cases (400 responses specifically), so it
// does not happen by default. Enabling checking will produce an error for each request that is not in accordance with
// the specification for that endpoint.
func WithRequestValidation() Option {
	return func(c *config) {
		c.checkRequest = true
	}
}

// WithoutFullCoverage is a functional Option for disabling verification that full coverage of the API has been
// accomplished. Full coverage is defined as having a test covering all documented response codes for all documented
// endpoint paths and methods. Using this option will still verify that no undocumented endpoints have been hit, as
// well as checking schemas for all valid interactions.
func WithoutFullCoverage() Option {
	return func(c *config) {
		c.disableFullCoverage = true
	}
}

// WithIgnoredUnsupportedBodyFormats is a functional Option to ignore unsupported body formats during response
// validation. Using this, only the supported bodies will be validated, and hitting more esoteric media types will not
// cause body validation to fail.
func WithIgnoredUnsupportedBodyFormats() Option {
	return func(c *config) {
		c.ignoreUnsupportedBodyFormats = true
	}
}

// RequestLogger is a minimal interface that can fit for example a testing.T, allowing tests to easily print logs where
// needed.
type RequestLogger interface {
	Logf(format string, args ...any)
}

// WithRequestLogging is a functional Option that provides a logger that copper will use to log out requests and
// responses. This can be useful for debugging, or writing initial tests for an endpoint, but will add quite a lot
// of log output for larger test suites.
func WithRequestLogging(l RequestLogger) Option {
	return func(c *config) {
		c.requestLogger = l
	}
}
