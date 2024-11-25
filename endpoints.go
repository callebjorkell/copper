package copper

import (
	"strings"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type methods struct {
	methods map[string]responses
}

type responses struct {
	responses map[string]bool
}

type endpoints struct {
	paths                     map[string]methods
	checkInternalServerErrors bool
}

func newEndpoints(model *v3.Document, checkInternalServerErrors bool) *endpoints {
	e := &endpoints{
		paths:                     make(map[string]methods),
		checkInternalServerErrors: checkInternalServerErrors,
	}

	e.loadPaths(model)
	return e
}

func (e *endpoints) loadPaths(model *v3.Document) {
	for path, pathItem := range model.Paths.PathItems.FromOldest() {
		e.loadPath(path, pathItem)
	}
}

func (e *endpoints) loadPath(path string, i *v3.PathItem) {
	if _, ok := e.paths[path]; !ok {
		e.paths[path] = methods{
			methods: make(map[string]responses),
		}
	}

	for method, op := range i.GetOperations().FromNewest() {
		method = strings.ToUpper(method)
		if _, ok := e.paths[path].methods[method]; !ok {
			e.paths[path].methods[method] = responses{
				responses: make(map[string]bool),
			}
		}

		if op.Responses != nil {
			for responseCode := range op.Responses.Codes.KeysFromNewest() {
				if !e.checkInternalServerErrors && responseCode == "500" {
					continue
				}

				e.paths[path].methods[method].responses[responseCode] = false
			}
		}
	}
}

// Endpoint represents a single coordinate in the endpoints tree.
type Endpoint struct {
	Path         string
	Method       string
	ResponseCode string
}

func (e *endpoints) responseMap(path, method string) map[string]bool {
	p, ok := e.paths[path]
	if !ok {
		return nil
	}

	method = strings.ToUpper(method)
	m, ok := p.methods[method]
	if !ok {
		return nil
	}

	return m.responses
}

func (e *endpoints) IsChecked(path, method, resCode string) bool {
	r := e.responseMap(path, method)
	return r[resCode]
}

// Unchecked returns a list of Endpoint entries, that all represent a coordinate in the endpoints tree that has not been
// marked as checked.
func (e *endpoints) Unchecked() []Endpoint {
	var ends []Endpoint

	for path, m := range e.paths {
		for method, r := range m.methods {
			for resCode, checked := range r.responses {
				if !checked {
					ends = append(ends, Endpoint{
						Path:         path,
						Method:       method,
						ResponseCode: resCode,
					})
				}
			}
		}
	}
	return ends
}

// MarkChecked will set an endpoint as checked, but only if it has been previously inserted. Will return false if
// no endpoint is present for the coordinate. Returns true even if the endpoint was previously checked.
func (e *endpoints) MarkChecked(path, method, resCode string) bool {
	r := e.responseMap(path, method)
	if r == nil {
		return false
	}

	r[resCode] = true
	return true
}
