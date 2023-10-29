package copper

import (
	"bytes"
	"context"
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

// NewVerifier takes bytes for an OpenAPI spec and a base path, and then returns a new Verifier that contains the
// declared paths.
func NewVerifier(specBytes []byte, basePath string) (*Verifier, error) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(specBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse spec data: %w", err)
	}

	if err := spec.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("schema is not valid: %w", err)
	}

	c := Verifier{
		base: strings.TrimRight(basePath, "/"),
		spec: spec,
	}

	for path, item := range spec.Paths {
		c.loadPath(path, item)
	}

	return &c, nil
}

func (v *Verifier) Record(res *http.Response) {
	req := res.Request
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
			reqInput := &openapi3filter.RequestValidationInput{
				Request: req,
				Route:   end.route,
				Options: &openapi3filter.Options{
					MultiError: true,
				},
			}

			if err := openapi3filter.ValidateRequest(context.Background(), reqInput); err != nil {
				v.errors = append(v.errors, fmt.Errorf("request invalid: %w", err))
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
				v.errors = append(v.errors, fmt.Errorf("response invalid: %w", err))
			}
			if bodyBytes.Len() > 0 {
				res.Body = io.NopCloser(&bodyBytes)
			}

			end.checked = true
			return
		}
	}

	v.errors = append(v.errors,
		fmt.Errorf("%v %v with response %v is not part of the spec", req.Method, req.URL.Path, res.StatusCode))
}

func (v *Verifier) Verify(t *testing.T) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, e := range v.errors {
		t.Error(e)
	}
	for i := range v.endpoints {
		if !v.endpoints[i].checked {
			t.Errorf("%v %v was not checked for response %v", v.endpoints[i].method, v.endpoints[i].path, v.endpoints[i].response)
		}
	}
}

func (v *Verifier) loadPath(path string, i *openapi3.PathItem) {
	re, _ := regexp.Compile("{[^}]*}")
	uri := fmt.Sprintf("^%v%v$", v.base, re.ReplaceAllLiteralString(path, "[^/]+"))
	uriRe, _ := regexp.Compile(uri)

	for _, method := range supportedMethods {
		op := i.GetOperation(method)
		if op == nil {
			continue
		}

		for responseCode := range op.Responses {
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
}
