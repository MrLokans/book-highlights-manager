package tasks

import "time"

// Config holds configuration for the task queue system.
type Config struct {
	// Workers is the number of concurrent task workers. Default: 2
	Workers int

	// MaxRetries is the default maximum retry attempts for failed tasks. Default: 3
	MaxRetries int

	// RetryDelay is the default backoff duration between retries. Default: 1m
	RetryDelay time.Duration

	// TaskTimeout is the default timeout for task execution. Default: 5m
	TaskTimeout time.Duration

	// ReleaseAfter is when stuck tasks are released back to queue. Default: 15m
	ReleaseAfter time.Duration

	// CleanupInterval is how often to clean up completed tasks. Default: 1h
	CleanupInterval time.Duration

	// RetentionDuration is how long to keep completed tasks. Default: 24h
	RetentionDuration time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Workers:           2,
		MaxRetries:        3,
		RetryDelay:        1 * time.Minute,
		TaskTimeout:       5 * time.Minute,
		ReleaseAfter:      15 * time.Minute,
		CleanupInterval:   1 * time.Hour,
		RetentionDuration: 24 * time.Hour,
	}
}
