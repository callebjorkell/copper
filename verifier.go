package copper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
)

var supportedMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPut,
	http.MethodPost,
	http.MethodDelete,
	http.MethodPatch,
	http.MethodOptions,
}

type endpoint struct {
	uriRe    *regexp.Regexp
	path     string
	checked  bool
	method   string
	response string
	route    *routers.Route
}

type Verifier struct {
	endpoints  []endpoint
	spec       *openapi3.T
	errors     []error
	conf       config
	mu         sync.Mutex
	reqCounter atomic.Int64
}

// NewVerifier takes bytes for an OpenAPI spec and options, and then returns a new Verifier for the given spec. Supply
// zero or more Option instances to change the behaviour of the Verifier.
func NewVerifier(specBytes []byte, opts ...Option) (*Verifier, error) {
	spec, err := loadSpec(specBytes)
	if err != nil {
		return nil, err
	}

	v := &Verifier{
		conf: getConfig(opts...),
		spec: spec,
	}

	if err := v.loadPaths(); err != nil {
		return nil, err
	}

	return v, nil
}

func loadSpec(specBytes []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(specBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse spec data: %w", err)
	}

	if err := spec.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("schema is not valid: %w", err)
	}
	return spec, nil
}

// loadPaths the paths into the data structure used verification
// if lenient is set, internal errors will be skipped.
func (v *Verifier) loadPaths() error {
	if v.spec == nil {
		return fmt.Errorf("spec is nil")
	}

	for path, item := range v.spec.Paths.Map() {
		err := v.loadPath(path, item)
		if err != nil {
			return fmt.Errorf("unable to loadPaths path %q: %w", path, err)
		}
	}

	return nil
}

func (v *Verifier) Record(res *http.Response) {
	req := res.Request

	// The body has already been read, so try to reset the body since kin-openapi expects this to be readable
	if req.GetBody != nil {
		req.Body, _ = req.GetBody()
	}

	if v.conf.requestLogger != nil {
		count := v.reqCounter.Add(1)
		reqDump, err := httputil.DumpRequestOut(req, true)
		if err == nil {
			v.conf.requestLogger.Logf("REQUEST  %04d ====\n%s", count, string(reqDump))
		}

		resDump, err := httputil.DumpResponse(res, true)
		if err == nil {
			v.conf.requestLogger.Logf("RESPONSE %04d ====\n%s", count, string(resDump))
		}
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	for i := range v.endpoints {
		end := &v.endpoints[i]
		if end.method != req.Method {
			continue
		}

		if end.response != strconv.Itoa(res.StatusCode) {
			continue
		}

		if end.uriRe.MatchString(req.URL.EscapedPath()) {
			matches := end.uriRe.FindStringSubmatch(req.URL.EscapedPath())
			params := make(map[string]string)
			for i, name := range end.uriRe.SubexpNames() {
				params[name] = matches[i]
			}

			reqInput := &openapi3filter.RequestValidationInput{
				Request:    req,
				Route:      end.route,
				PathParams: params,
				Options: &openapi3filter.Options{
					MultiError: true,
				},
			}

			if v.conf.checkRequest {
				if err := openapi3filter.ValidateRequest(context.Background(), reqInput); err != nil {
					v.errors = append(
						v.errors,
						joinError(ErrRequestInvalid, fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)),
					)
				}
			}

			bodyBytes := bytes.Buffer{}
			bodyTee := io.TeeReader(res.Body, &bodyBytes)
			responseErr := openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
				RequestValidationInput: reqInput,
				Status:                 res.StatusCode,
				Header:                 res.Header,
				Body:                   io.NopCloser(bodyTee),
				Options:                reqInput.Options,
			})

			// reset the body.
			if bodyBytes.Len() > 0 {
				res.Body = io.NopCloser(&bodyBytes)
			}

			end.checked = true

			if responseErr != nil {
				var parseErr *openapi3filter.ParseError
				if v.conf.ignoreUnsupportedBodyFormats && errors.As(responseErr, &parseErr) {
					if parseErr.Kind == openapi3filter.KindUnsupportedFormat {
						// openapi3filter doesn't support the format, and we've elected to ignore those bodies, so just
						// ignore the error and return.
						return
					}
				}

				v.errors = append(
					v.errors,
					joinError(ErrResponseInvalid, fmt.Errorf("%s %s: %d: %w", req.Method, req.URL.Path, res.StatusCode, responseErr)),
				)
			}

			return
		}
	}

	if v.conf.checkInternalServerErrors || res.StatusCode != http.StatusInternalServerError {
		v.errors = append(
			v.errors,
			joinError(ErrNotPartOfSpec, fmt.Errorf("%v %v: %v", req.Method, req.URL.Path, res.StatusCode)),
		)
	}
}

// CurrentError is a convenience method for CurrentErrors, where the errors are joined into a single error, making
// it easier to check.
func (v *Verifier) CurrentError() error {
	return errors.Join(v.CurrentErrors()...)
}

// CurrentErrors return the current collection of errors in the verifier.
func (v *Verifier) CurrentErrors() []error {
	v.mu.Lock()
	defer v.mu.Unlock()

	var errs []error
	if !v.conf.disableFullCoverage {
		for i := range v.endpoints {
			if !v.endpoints[i].checked {
				err := fmt.Errorf("%s %s: %s", v.endpoints[i].method, v.endpoints[i].path, v.endpoints[i].response)
				errs = append(errs, joinError(ErrNotChecked, err))
			}
		}
	}

	return append(v.errors, errs...)
}

// Verify will cause the given test context to fail with an error if Error returns a non-nil error.
func (v *Verifier) Verify(t *testing.T) {
	err := v.CurrentError()
	if err != nil {
		t.Error(err)
	}
}

func (v *Verifier) loadPath(path string, i *openapi3.PathItem) error {
	// Turn the path into a regular expression with named capture groups corresponding to the name of the parameter
	// in the spec.
	re := regexp.MustCompile("{([^}]*)}")
	uri := fmt.Sprintf("^%v%v$", v.conf.basePath, re.ReplaceAllString(path, "(?P<$1>[^/]+)"))
	uriRe, err := regexp.Compile(uri)
	if err != nil {
		return fmt.Errorf("could not compile regex for %v: %w", path, err)
	}

	for _, method := range supportedMethods {
		op := i.GetOperation(method)
		if op == nil {
			continue
		}

		for responseCode := range op.Responses.Map() {
			if !v.conf.checkInternalServerErrors && responseCode == "500" {
				continue
			}

			e := endpoint{
				checked:  false,
				method:   method,
				response: responseCode,
				uriRe:    uriRe,
				path:     v.conf.basePath + path,
				route: &routers.Route{
					Spec:      v.spec,
					PathItem:  i,
					Method:    method,
					Operation: op,
				},
			}
			v.endpoints = append(v.endpoints, e)
		}
	}

	return nil
}
