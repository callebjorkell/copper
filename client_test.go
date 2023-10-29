package copper

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWithBasePath(t *testing.T) {
	tt := []struct {
		path string
		want string
	}{
		{"", ""},
		{"/something", "/something"},
		{"/some/thing/", "/some/thing/"},
	}

	for _, tc := range tt {
		t.Run(tc.path, func(t *testing.T) {
			c := &config{}
			opt := WithBasePath(tc.path)

			opt(c)

			assert.Equal(t, tc.want, c.base)
		})
	}
}
