package auth

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter provides rate limiting for login attempts.
// It tracks failed attempts per IP+username combination using a sliding window.
type RateLimiter struct {
	mu              sync.RWMutex
	attempts        map[string]*attemptRecord
	maxAttempts     int
	windowDuration  time.Duration
	lockoutDuration time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

type attemptRecord struct {
	count      int
	firstAttempt time.Time
	lockedUntil  time.Time
}

// RateLimitConfig contains configuration for the rate limiter.
type RateLimitConfig struct {
	MaxAttempts     int           // Maximum attempts before lockout (default: 5)
	WindowDuration  time.Duration // Time window for counting attempts (default: 15m)
	LockoutDuration time.Duration // How long to lock out after max attempts (default: 30m)
	CleanupInterval time.Duration // How often to clean up expired records (default: 5m)
}

// DefaultRateLimitConfig returns sensible defaults for rate limiting.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxAttempts:     5,
		WindowDuration:  15 * time.Minute,
		LockoutDuration: 30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.WindowDuration <= 0 {
		cfg.WindowDuration = 15 * time.Minute
	}
	if cfg.LockoutDuration <= 0 {
		cfg.LockoutDuration = 30 * time.Minute
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		attempts:        make(map[string]*attemptRecord),
		maxAttempts:     cfg.MaxAttempts,
		windowDuration:  cfg.WindowDuration,
		lockoutDuration: cfg.LockoutDuration,
		cleanupInterval: cfg.CleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup
	go rl.cleanupLoop()

	return rl
}

// Stop stops the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// makeKey creates a unique key for IP+username combination.
func (rl *RateLimiter) makeKey(ip, username string) string {
	return ip + ":" + username
}

// Allow checks if a login attempt should be allowed.
// Returns (allowed bool, retryAfter time.Duration).
// If not allowed, retryAfter indicates when the lockout expires.
func (rl *RateLimiter) Allow(ip, username string) (bool, time.Duration) {
	key := rl.makeKey(ip, username)
	now := time.Now()

	rl.mu.RLock()
	record, exists := rl.attempts[key]
	rl.mu.RUnlock()

	if !exists {
		return true, 0
	}

	// Check if currently locked out
	if !record.lockedUntil.IsZero() && now.Before(record.lockedUntil) {
		return false, record.lockedUntil.Sub(now)
	}

	// Check if window has expired (reset)
	if now.Sub(record.firstAttempt) > rl.windowDuration {
		return true, 0
	}

	// Check if under limit
	if record.count < rl.maxAttempts {
		return true, 0
	}

	return false, rl.lockoutDuration
}

// RecordFailure records a failed login attempt.
// Returns (locked bool, retryAfter time.Duration) indicating if account is now locked.
func (rl *RateLimiter) RecordFailure(ip, username string) (bool, time.Duration) {
	key := rl.makeKey(ip, username)
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	record, exists := rl.attempts[key]
	if !exists {
		record = &attemptRecord{
			count:        0,
			firstAttempt: now,
		}
		rl.attempts[key] = record
	}

	// Reset if window expired
	if now.Sub(record.firstAttempt) > rl.windowDuration {
		record.count = 0
		record.firstAttempt = now
		record.lockedUntil = time.Time{}
	}

	record.count++

	// Check if this triggers a lockout
	if record.count >= rl.maxAttempts {
		record.lockedUntil = now.Add(rl.lockoutDuration)
		return true, rl.lockoutDuration
	}

	return false, 0
}

// RecordSuccess clears the failure record for a successful login.
func (rl *RateLimiter) RecordSuccess(ip, username string) {
	key := rl.makeKey(ip, username)

	rl.mu.Lock()
	delete(rl.attempts, key)
	rl.mu.Unlock()
}

// cleanupLoop periodically removes expired records.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes expired records.
func (rl *RateLimiter) cleanup() {
	now := time.Now()
	expiry := rl.windowDuration + rl.lockoutDuration

	rl.mu.Lock()
	defer rl.mu.Unlock()

	for key, record := range rl.attempts {
		// Remove if both window and lockout have expired
		windowExpired := now.Sub(record.firstAttempt) > expiry
		lockoutExpired := record.lockedUntil.IsZero() || now.After(record.lockedUntil)

		if windowExpired && lockoutExpired {
			delete(rl.attempts, key)
		}
	}
}

// RateLimitMiddleware creates Gin middleware for rate limiting login attempts.
// It should be applied only to the login route.
func (rl *RateLimiter) RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only check on POST (actual login attempt)
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		// Get IP and username
		ip := c.ClientIP()
		username := c.PostForm("username")
		if username == "" {
			c.Next()
			return
		}

		// Check rate limit
		allowed, retryAfter := rl.Allow(ip, username)
		if !allowed {
			c.Header("Retry-After", retryAfter.String())
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "too many login attempts",
				"retry_after": retryAfter.String(),
			})
			return
		}

		c.Next()
	}
}

// GetClientIP extracts the client IP from a gin context (exported for use in handlers).
func GetClientIP(c *gin.Context) string {
	return c.ClientIP()
}
