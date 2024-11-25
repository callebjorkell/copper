package copper

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	validatorerr "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi-validator/paths"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type Verifier struct {
	endpoints  *endpoints
	errors     []error
	conf       config
	mu         sync.Mutex
	reqCounter atomic.Int64
	validator  validator.Validator
	model      *v3.Document
}

// NewVerifier takes bytes for an OpenAPI spec and options, and then returns a new Verifier for the given spec. Supply
// zero or more Option instances to change the behaviour of the Verifier.
func NewVerifier(specBytes []byte, opts ...Option) (*Verifier, error) {
	spec, err := libopenapi.NewDocument(specBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse spec data: %w", err)
	}

	ok, validationErrs := schema_validation.ValidateOpenAPIDocument(spec)
	if !ok {
		return nil, fmt.Errorf("schema is not valid: %w", toError(validationErrs))
	}

	model, errs := spec.BuildV3Model()
	if len(errs) > 0 {
		return nil, fmt.Errorf("unable to create model: %w", errors.Join(errs...))
	}

	conf := getConfig(opts...)
	if conf.serverBase != "" {
		model.Model.Servers = []*v3.Server{
			{
				URL:         conf.serverBase,
				Description: "Added by copper option",
			},
		}
	}

	docValidator := validator.NewValidatorFromV3Model(&model.Model)

	var v = &Verifier{
		conf:      conf,
		validator: docValidator,
		model:     &model.Model,
		endpoints: newEndpoints(&model.Model, conf.checkInternalServerErrors),
	}

	return v, nil
}

func (v *Verifier) check(req *http.Request, res *http.Response) {
	_, errs, foundPath := paths.FindPath(req, v.model)
	if len(errs) > 0 {
		v.appendErr(ErrNotPartOfSpec, fmt.Errorf("%v %v: %v", req.Method, req.URL.Path, toError(errs)))
		return
	}

	v.endpoints.MarkChecked(foundPath, req.Method, strconv.Itoa(res.StatusCode))

	// Select the right function for validation.
	if v.conf.checkRequest {
		ok, validationErrors := v.validator.ValidateHttpRequest(req)
		if !ok {
			v.appendErr(ErrRequestInvalid, fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, toError(validationErrors)))
		}
	}

	ok, validationErrors := v.validator.ValidateHttpResponse(req, res)
	if !ok {
		v.appendErr(ErrResponseInvalid, fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, toError(validationErrors)))
	}
}

func (v *Verifier) appendErr(sentinel SentinelError, err error) {
	v.errors = append(
		v.errors,
		joinError(sentinel, err),
	)
}

func (v *Verifier) Record(res *http.Response) {
	req := res.Request

	// The body has already been read, so try to reset the body
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

	v.check(req, res)
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
		for _, e := range v.endpoints.Unchecked() {
			err := fmt.Errorf("%s %s: %s", e.Method, e.Path, e.ResponseCode)
			errs = append(errs, joinError(ErrNotChecked, err))
		}
	}

	return append(v.errors, errs...)
}

// Verify will cause the given test context to fail with an error if Error returns a non-nil error.
func (v *Verifier) Verify(t *testing.T) {
	t.Helper()

	err := v.CurrentError()
	if err != nil {
		t.Error(err)
	}
}

// Reset will remove all current errors, and start the Verifier from scratch. This allows it to be reused.
func (v *Verifier) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.errors = nil
	v.endpoints = newEndpoints(v.model, v.conf.checkInternalServerErrors)
}

func toError(validationErrs []*validatorerr.ValidationError) error {
	if len(validationErrs) == 1 {
		return validationErrs[0]
	}

	s := strings.Builder{}
	for _, err := range validationErrs {
		s.WriteString("\n - ")
		s.WriteString(err.Error())
	}
	return fmt.Errorf("validation errors: %s", s.String())
}
