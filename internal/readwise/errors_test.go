package readwise

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerError(t *testing.T) {
	err := &ServerError{StatusCode: 500}
	assert.Equal(t, "Readwise server error: HTTP 500", err.Error())

	err502 := &ServerError{StatusCode: 502}
	assert.Contains(t, err502.Error(), "502")
}

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrInvalidToken)
	assert.Error(t, ErrRateLimited)
	assert.Contains(t, ErrInvalidToken.Error(), "invalid")
	assert.Contains(t, ErrRateLimited.Error(), "rate limit")
}
