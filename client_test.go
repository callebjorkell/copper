package copper

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestWithBasePath(t *testing.T) {
	tt := []struct {
		path string
		want string
	}{
		{"", "/"},
		{"/something", "/something"},
		{"/some/thing/", "/some/thing"},
		{"somebase", "/somebase"},
	}

	for _, tc := range tt {
		t.Run(tc.path, func(t *testing.T) {
			c := &config{}
			opt := WithBasePath(tc.path)

			opt(c)

			assert.Equal(t, tc.want, c.basePath)
		})
	}
}

func TestDifferentBase(t *testing.T) {
	f, err := os.Open("testdata/delete-spec.yaml")
	require.NoError(t, err)
	defer f.Close()

	s := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/mybase/thing/10" {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}),
	)
	defer s.Close()

	c, err := WrapClient(http.DefaultClient, f, WithBasePath("/mybase"))
	require.NoError(t, err)

	_, err = c.Delete(s.URL + "/mybase/thing/10")
	assert.NoError(t, err)

	c.Verify(t)
}

func TestParamValidation(t *testing.T) {
	f, err := os.ReadFile("testdata/param-spec.yaml")
	require.NoError(t, err)

	s := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	defer s.Close()

	tt := []struct {
		name   string
		rName  string
		age    string
		gender string
		valid  bool
	}{
		{"too short name", "Bob", "2", "other", false},
		{"empty name", "", "2", "other", false},
		{"invalid age", "Bobaloo", "-1", "other", false},
		{"fine female", "Mrs Bobaloo", "105", "female", true},
		{"fine male", "Bobaloo", "62", "male", true},
		{"fine other", "Bobaloo", "62", "other", true},
		{"bad alien", "Bobaloo", "62", "alien", false},
		{"empty gender", "Bobaloo", "62", "", false},
		{"empty age", "Bobaloo", "", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c, err := WrapClient(http.DefaultClient, bytes.NewReader(f), WithRequestValidation())
			require.NoError(t, err)

			_, err = c.Head(fmt.Sprintf("%s/%s/%s/%s", s.URL, tc.rName, tc.age, tc.gender))
			assert.NoError(t, err)

			if tc.valid {
				assert.Empty(t, c.CurrentErrors())
			} else {
				assert.NotEmpty(t, c.CurrentErrors())
			}
		})
	}
}

func TestWrapClient(t *testing.T) {
	f, err := os.Open("testdata/thing-spec.yaml")
	require.NoError(t, err)
	defer f.Close()

	s := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if r.URL.Path == "/ping" {
				_, _ = w.Write([]byte(`{"message":"pong!"}`))
			} else {
				_, _ = w.Write([]byte(`{"thing": "yes"}`))
			}
		}),
	)
	defer s.Close()

	c, err := WrapClient(http.DefaultClient, f)
	require.NoError(t, err)

	_, err = c.Get(s.URL + "/ping")
	assert.NoError(t, err)

	other, err := c.WithClient(&http.Client{})
	require.NoError(t, err)

	_, err = other.Get(s.URL + "/other")
	assert.NoError(t, err)

	c.Verify(t)
}

type numberHandler struct {
	contentType string
	path        string
	number      string
}

func (n *numberHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", n.contentType)
	w.WriteHeader(http.StatusOK)
	if r.URL.Path == n.path {
		_, _ = w.Write([]byte(fmt.Sprintf("{\"number\": %v}", n.number)))
	}
}

type logStore struct {
	mu   sync.Mutex
	logs []string
}

func (l *logStore) Logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprintf(format, args...))
}

func TestRequestLogging(t *testing.T) {
	f, err := os.ReadFile("testdata/minimal-spec.yaml")
	require.NoError(t, err)

	s := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	r := bytes.NewReader(f)
	store := &logStore{}
	c, err := WrapClient(http.DefaultClient, r, WithRequestLogging(store))
	require.NoError(t, err)

	_, err = c.Get(s.URL + "/ping")
	require.NoError(t, err)

	if assert.Len(t, store.logs, 2) {
		for _, log := range store.logs {
			assert.NotEmpty(t, log)
		}
	}

	c.Verify(t)
}

func TestValidationErrors(t *testing.T) {
	f, err := os.ReadFile("testdata/number-spec.yaml")
	require.NoError(t, err)

	tt := []struct {
		name        string
		contentType string
		number      string
		requestPath string
	}{
		{"wrong path", "application/json", "2", "/wrong"},
		{"not a number", "application/json", "two", "/mini"},
		{"base path", "application/json", "2", "/"},
		{"no content type", "", "2", "/mini"},
		{"wrong content type", "text/plain", "2", "/mini"},
		{"empty number", "application/json", "", "/mini"},
	}

	handler := &numberHandler{}
	s := httptest.NewServer(handler)
	defer s.Close()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			handler.path, handler.contentType, handler.number = tc.requestPath, tc.contentType, tc.number

			r := bytes.NewReader(f)
			c, err := WrapClient(http.DefaultClient, r)
			require.NoError(t, err)

			url := fmt.Sprintf("%s%s", s.URL, tc.requestPath)
			_, err = c.Get(url)
			assert.NoError(t, err)

			// filter away any "not checked errors", and only check the other ones.
			errs := c.CurrentErrors()
			i := 0
			for _, err := range errs {
				if !errors.Is(err, ErrNotChecked) {
					errs[i] = err
					i++
				}
			}
			errs = errs[:i]
			assert.NotEmpty(t, errs)
		})
	}
}

func TestRequestBodyValidation(t *testing.T) {
	f, err := os.ReadFile("testdata/request-body-spec.yaml")
	require.NoError(t, err)

	s := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	defer s.Close()

	tt := []struct {
		name        string
		contentType string
		body        string
		shouldError bool
	}{
		{"according to spec", "application/json", `{"input":"pem"}`, false},
		{"wrong content type", "text/plain", `{"input":"pem"}`, true},
		{"wrong input field type", "application/json", `{"input":5}`, true},
		{"missing input field", "application/json", `{"message":"stuff"}`, true},
		{"extra fields in body", "application/json", `{"input": "yes", "message":"stuff"}`, false},
		{"empty content type", "", `{"input":"pem"}`, true},
		{"empty body", "application/json", "", true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(f)
			c, err := WrapClient(http.DefaultClient, r, WithRequestValidation())
			require.NoError(t, err)

			res, err := c.Post(s.URL+"/req", tc.contentType, strings.NewReader(tc.body))
			require.Equal(t, 204, res.StatusCode)

			assert.NoError(t, err)
			if tc.shouldError {
				current := c.CurrentErrors()
				if assert.NotEmpty(t, current) {
					joined := errors.Join(current...)
					assert.ErrorIs(t, joined, ErrRequestInvalid)
				}
			} else {
				assert.Empty(t, c.CurrentErrors())
			}
		})
	}
}
