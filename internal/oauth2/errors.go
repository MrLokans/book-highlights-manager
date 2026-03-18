// Package oauth2 implements OAuth2 authorization flows and token management.
package oauth2

import "errors"

// OAuth2 error sentinels.
var (
	ErrTokenNotFound    = errors.New("token not found")
	ErrNoRefreshToken   = errors.New("no refresh token available")
	ErrTokenExpired     = errors.New("token expired")
	ErrProviderNotFound = errors.New("provider not registered")
)
