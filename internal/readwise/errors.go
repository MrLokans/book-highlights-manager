package readwise

import (
	"errors"
	"fmt"
)

// ErrInvalidToken indicates the provided API token is invalid
var ErrInvalidToken = errors.New("invalid or expired Readwise token")

// ErrRateLimited indicates the API rate limit was exceeded
var ErrRateLimited = errors.New("readwise API rate limit exceeded")

// ServerError represents a 5xx error from the Readwise API
type ServerError struct {
	StatusCode int
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("Readwise server error: HTTP %d", e.StatusCode)
}
