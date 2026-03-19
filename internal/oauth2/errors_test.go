package oauth2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrTokenNotFound)
	assert.Error(t, ErrNoRefreshToken)
	assert.Error(t, ErrTokenExpired)
	assert.Error(t, ErrProviderNotFound)
	assert.Contains(t, ErrTokenNotFound.Error(), "not found")
	assert.Contains(t, ErrNoRefreshToken.Error(), "no refresh token")
}
