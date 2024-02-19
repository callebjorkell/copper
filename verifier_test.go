package copper

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentErrors(t *testing.T) {
	f, err := os.ReadFile("testdata/thing-spec.yaml")
	require.NoError(t, err)

	v, err := NewVerifier(f)
	require.NoError(t, err)

	errs := v.CurrentErrors()
	assert.NotEmpty(t, errs)
}

func TestCurrentError(t *testing.T) {
	f, err := os.ReadFile("testdata/delete-spec.yaml")
	require.NoError(t, err)

	v, err := NewVerifier(f)
	require.NoError(t, err)

	t.Run("errors can be joined", func(t *testing.T) {
		assert.ErrorIs(t, v.CurrentError(), ErrNotChecked)
	})

	t.Run("errors can be nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/thing/19", nil)
		v.Record(&http.Response{StatusCode: 204, Request: req})
		assert.NoError(t, v.CurrentError())
	})
}

func TestWithInternalServerErrors(t *testing.T) {
	f, err := os.ReadFile("testdata/server-error-spec.yaml")
	require.NoError(t, err)

	t.Run("unchecked 500 returns error when including server errors", func(t *testing.T) {
		t.Parallel()
		v, err := NewVerifier(f, WithInternalServerErrors())
		require.NoError(t, err)
		assert.ErrorIs(t, v.CurrentError(), ErrNotChecked)
	})

	t.Run("unchecked 500 is fine", func(t *testing.T) {
		t.Parallel()
		v, err := NewVerifier(f)
		require.NoError(t, err)
		assert.NoError(t, v.CurrentError())
	})

	t.Run("checked 500 is fine", func(t *testing.T) {
		t.Parallel()
		v, err := NewVerifier(f, WithInternalServerErrors())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/fault", nil)
		v.Record(&http.Response{StatusCode: 500, Request: req})

		assert.NoError(t, v.CurrentError())
	})
}

func TestWithRequestValidation(t *testing.T) {
	bodySpec, err := os.ReadFile("testdata/request-body-spec.yaml")
	require.NoError(t, err)
	queryParamSpec, err := os.ReadFile("testdata/query-param-spec.yaml")
	require.NoError(t, err)

	t.Run("invalid body fails validation when checked", func(t *testing.T) {
		v, err := NewVerifier(bodySpec, WithRequestValidation())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/req", strings.NewReader(`{"borken": "yes"}`))
		req.Header.Set("Content-Type", "application/json")
		v.Record(&http.Response{StatusCode: 204, Request: req})

		assert.ErrorIs(t, v.CurrentError(), ErrRequestInvalid)
	})

	t.Run("invalid body succeeds when not checked", func(t *testing.T) {
		v, err := NewVerifier(bodySpec)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/req", strings.NewReader(`{"borken": "yes"}`))
		req.Header.Set("Content-Type", "application/json")
		v.Record(&http.Response{StatusCode: 204, Request: req})

		assert.NoError(t, v.CurrentError())
	})

	t.Run("invalid query param succeeds when not checked", func(t *testing.T) {
		v, err := NewVerifier(queryParamSpec)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/req?id=1", nil)
		req.Header.Set("Content-Type", "application/json")
		v.Record(&http.Response{StatusCode: 204, Request: req})

		assert.NoError(t, v.CurrentError())
	})

	t.Run("invalid query param fails validation when checked", func(t *testing.T) {
		v, err := NewVerifier(queryParamSpec, WithRequestValidation())
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/req?id=1", nil)
		req.Header.Set("Content-Type", "application/json")
		v.Record(&http.Response{StatusCode: 204, Request: req})

		assert.ErrorIs(t, v.CurrentError(), ErrRequestInvalid)
	})

	t.Run("disabling full coverage validation allows untested endpoints", func(t *testing.T) {
		v, err := NewVerifier(queryParamSpec, WithoutFullCoverage())
		require.NoError(t, err)
		assert.NoError(t, v.CurrentError())
	})

	t.Run("disabling full coverage validation does not allow undocumented requests", func(t *testing.T) {
		v, err := NewVerifier(queryParamSpec, WithoutFullCoverage())

		req := httptest.NewRequest(http.MethodGet, "/some-other-path", nil)
		req.Header.Set("Content-Type", "application/json")
		v.Record(&http.Response{StatusCode: 204, Request: req})

		require.NoError(t, err)
		assert.ErrorIs(t, v.CurrentError(), ErrNotPartOfSpec)
	})
}
