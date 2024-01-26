package copper

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCurrentErrors(t *testing.T) {
	f, err := os.ReadFile("testdata/thing-spec.yaml")
	require.NoError(t, err)

	v, err := NewVerifier(f, "")
	require.NoError(t, err)

	errs := v.CurrentErrors()
	if assert.NotEmpty(t, errs) {

	}
}

func TestCurrentError(t *testing.T) {
	f, err := os.ReadFile("testdata/delete-spec.yaml")
	require.NoError(t, err)

	v, err := NewVerifier(f, "")
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
