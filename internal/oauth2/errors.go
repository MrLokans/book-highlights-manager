package oauth2

import "errors"

var (
	ErrTokenNotFound   = errors.New("token not found")
	ErrNoRefreshToken  = errors.New("no refresh token available")
	ErrTokenExpired    = errors.New("token expired")
	ErrProviderNotFound = errors.New("provider not registered")
)
