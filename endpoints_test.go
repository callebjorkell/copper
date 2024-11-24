package copper

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEndpoints_MarkChecked(t *testing.T) {
	e := &endpoints{
		paths: map[string]methods{
			"/study/my/{id}": {
				methods: map[string]responses{
					http.MethodPut: {responses: map[string]bool{
						"200": true,
					}},
				},
			},
			"/study/other/{id}": {
				methods: map[string]responses{
					http.MethodGet: {responses: map[string]bool{
						"200": false,
						"404": false,
					}},
				},
			},
		},
	}

	tt := []struct {
		name     string
		path     string
		method   string
		resCode  string
		expected bool
	}{
		{
			"checked endpoint can be marked again",
			"/study/my/{id}",
			http.MethodPut,
			"200",
			true,
		},
		{
			"missing endpoint will not be marked",
			"/other/endpoint",
			http.MethodPut,
			"201",
			false,
		},
		{
			"inserted endpoint can be marked",
			"/study/other/{id}",
			http.MethodGet,
			"200",
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			e.MarkChecked(tc.path, tc.method, tc.resCode)
			assert.Equal(t, tc.expected, e.IsChecked(tc.path, tc.method, tc.resCode))
		})
	}
}

func TestIsChecked(t *testing.T) {
	e := &endpoints{
		paths: map[string]methods{
			"/ping": {
				methods: map[string]responses{
					http.MethodPut: {responses: map[string]bool{
						"200": false,
					}},
				},
			},
			"/ping/{thevalue}": {
				methods: map[string]responses{
					http.MethodGet: {responses: map[string]bool{
						"200": false,
						"404": false,
						"401": true,
					}},
					http.MethodDelete: {responses: map[string]bool{
						"204": false,
					}},
				},
			},
		},
	}

	tt := []struct {
		name     string
		path     string
		method   string
		resCode  string
		expected bool
	}{
		{
			"check non-inserted path",
			"/foo",
			"GET",
			"200",
			false,
		},
		{
			"check checked path",
			"/ping/{thevalue}",
			"GET",
			"401",
			true,
		},
		{
			"check existing unchecked",
			"/ping/{thevalue}",
			"GET",
			"404",
			false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual := e.IsChecked(tc.path, tc.method, tc.resCode)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
