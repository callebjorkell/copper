package copper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
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
	endpoints []endpoint
	spec      *openapi3.T
	base      string
	errors    []error
	mu        sync.Mutex
}

// NewExactVerifier does not skip any part of the spec, for example internal server errors, and will require them to be
// checked if defined in the spec.
func NewExactVerifier(specBytes []byte, basePath string) (*Verifier, error) {
	v, err := newVerifier(specBytes, basePath)
	if err != nil {
		return v, err
	}

	if err := v.loadPaths(false); err != nil {
		return nil, err
	}
	return v, nil
}

// NewVerifier takes bytes for an OpenAPI spec and a base path, and then returns a new Verifier that contains the
// declared paths. NewVerifier will not check internal server errors even if they are included in the spec.
func NewVerifier(specBytes []byte, basePath string) (*Verifier, error) {
	v, err := newVerifier(specBytes, basePath)
	if err != nil {
		return v, err
	}

	if err := v.loadPaths(true); err != nil {
		return nil, err
	}
	return v, nil
}

func newVerifier(specBytes []byte, basePath string) (*Verifier, error) {
	spec, err := loadSpec(specBytes)
	if err != nil {
		return nil, err
	}

	v := Verifier{
		base: strings.TrimRight(basePath, "/"),
		spec: spec,
	}
	return &v, nil
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
func (v *Verifier) loadPaths(lenient bool) error {
	if v.spec == nil {
		return fmt.Errorf("spec is nil")
	}

	for path, item := range v.spec.Paths.Map() {
		err := v.loadPath(path, item, lenient)
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

			if err := openapi3filter.ValidateRequest(context.Background(), reqInput); err != nil {
				v.errors = append(
					v.errors,
					joinError(ErrRequestInvalid, fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)),
				)
			}

			bodyBytes := bytes.Buffer{}
			bodyTee := io.TeeReader(res.Body, &bodyBytes)
			err := openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
				RequestValidationInput: reqInput,
				Status:                 res.StatusCode,
				Header:                 res.Header,
				Body:                   io.NopCloser(bodyTee),
				Options:                reqInput.Options,
			})
			if err != nil {
				v.errors = append(
					v.errors,
					joinError(ErrResponseInvalid, fmt.Errorf("%s %s: %d: %w", req.Method, req.URL.Path, res.StatusCode, err)),
				)
			}
			if bodyBytes.Len() > 0 {
				res.Body = io.NopCloser(&bodyBytes)
			}

			end.checked = true
			return
		}
	}

	v.errors = append(
		v.errors,
		joinError(ErrNotPartOfSpec, fmt.Errorf("%v %v: %v", req.Method, req.URL.Path, res.StatusCode)),
	)
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
	for i := range v.endpoints {
		if !v.endpoints[i].checked {
			err := fmt.Errorf("%s %s: %s", v.endpoints[i].method, v.endpoints[i].path, v.endpoints[i].response)
			errs = append(errs, joinError(ErrNotChecked, err))
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

func (v *Verifier) loadPath(path string, i *openapi3.PathItem, lenient bool) error {
	// Turn the path into a regular expression with named capture groups corresponding to the name of the parameter
	// in the spec.
	re := regexp.MustCompile("{([^}]*)}")
	uri := fmt.Sprintf("^%v%v$", v.base, re.ReplaceAllString(path, "(?P<$1>[^/]+)"))
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
			if lenient && responseCode == "500" {
				continue
			}

			e := endpoint{
				checked:  false,
				method:   method,
				response: responseCode,
				uriRe:    uriRe,
				path:     v.base + path,
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
