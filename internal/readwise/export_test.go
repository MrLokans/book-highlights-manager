package readwise

import "time"

// overrideRetryDelay sets a custom base delay for retry backoff in tests.
func overrideRetryDelay(d time.Duration) {
	retryBaseDelay = d
}

// resetRetryDelay restores the default retry delay.
func resetRetryDelay(d time.Duration) {
	retryBaseDelay = d
}
