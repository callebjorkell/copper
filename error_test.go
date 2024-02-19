package copper

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinErrorIs(t *testing.T) {
	testErr := errors.New("my test error")
	t.Run("error is", func(t *testing.T) {
		err := joinError(ErrNotPartOfSpec, testErr)
		assert.ErrorIs(t, err, ErrNotPartOfSpec)
		assert.ErrorIs(t, err, testErr)
	})

	t.Run("error is nested", func(t *testing.T) {
		subErr := joinError(ErrNotPartOfSpec, testErr)
		err := joinError(ErrRequestInvalid, subErr)
		assert.ErrorIs(t, err, ErrNotPartOfSpec)
		assert.ErrorIs(t, err, ErrRequestInvalid)
		assert.ErrorIs(t, err, testErr)
	})
}
