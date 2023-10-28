package copper

import (
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
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
}

type Verifier struct {
	endpoints []endpoint
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

	c := Verifier{
		base: strings.TrimRight(basePath, "/"),
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
		if v.endpoints[i].method != req.Method {
			continue
		}

		if v.endpoints[i].response != strconv.Itoa(res.StatusCode) {
			continue
		}

		if v.endpoints[i].uriRe.MatchString(req.URL.EscapedPath()) {
			v.endpoints[i].checked = true
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
			}
			v.endpoints = append(v.endpoints, e)
		}
	}
}
