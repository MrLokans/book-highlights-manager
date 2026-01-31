package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestOpenRedirectPrevention verifies that redirect paths are properly sanitized.
func TestOpenRedirectPrevention(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty path", "", "/"},
		{"root path", "/", "/"},
		{"local path", "/dashboard", "/dashboard"},
		{"local path with query", "/search?q=test", "/search?q=test"},
		{"protocol-relative URL", "//evil.com", "/"},
		{"full URL with scheme", "https://evil.com", "/"},
		{"URL with scheme in path", "/https://evil.com", "/"}, // Contains :// so rejected for safety
		{"backslash escape attempt", "/foo\\bar", "/"},
		{"backslash at start", "\\evil.com", "/"},
		{"data URL", "data:text/html,<script>", "/"},
		{"javascript URL", "javascript:alert(1)", "/"},
		{"no leading slash", "evil.com", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRedirectPath(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeRedirectPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsLocalPath verifies local path detection.
func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty", "", false},
		{"root", "/", true},
		{"local path", "/foo/bar", true},
		{"protocol-relative", "//evil.com", false},
		{"full URL", "https://evil.com", false},
		{"no leading slash", "foo/bar", false},
		{"backslash", "/foo\\bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalPath(tt.input)
			if result != tt.expected {
				t.Errorf("isLocalPath(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestRateLimiter tests the rate limiting functionality.
func TestRateLimiter_AllowsInitialAttempts(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxAttempts:     3,
		WindowDuration:  time.Minute,
		LockoutDuration: time.Minute,
		CleanupInterval: time.Hour, // Long interval to prevent cleanup during test
	})
	defer rl.Stop()

	// First 3 attempts should be allowed
	for i := 0; i < 3; i++ {
		allowed, _ := rl.Allow("192.168.1.1", "testuser")
		if !allowed {
			t.Errorf("Attempt %d should be allowed", i+1)
		}
		rl.RecordFailure("192.168.1.1", "testuser")
	}

	// 4th attempt should be blocked
	allowed, retryAfter := rl.Allow("192.168.1.1", "testuser")
	if allowed {
		t.Error("4th attempt should be blocked")
	}
	if retryAfter == 0 {
		t.Error("retryAfter should be non-zero when blocked")
	}
}

func TestRateLimiter_SuccessResetsCounter(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxAttempts:     3,
		WindowDuration:  time.Minute,
		LockoutDuration: time.Minute,
		CleanupInterval: time.Hour,
	})
	defer rl.Stop()

	// Record 2 failures
	rl.RecordFailure("192.168.1.1", "testuser")
	rl.RecordFailure("192.168.1.1", "testuser")

	// Successful login
	rl.RecordSuccess("192.168.1.1", "testuser")

	// Should be allowed again (counter reset)
	allowed, _ := rl.Allow("192.168.1.1", "testuser")
	if !allowed {
		t.Error("Should be allowed after successful login")
	}
}

func TestRateLimiter_DifferentUsersAreIndependent(t *testing.T) {
	rl := NewRateLimiter(RateLimitConfig{
		MaxAttempts:     2,
		WindowDuration:  time.Minute,
		LockoutDuration: time.Minute,
		CleanupInterval: time.Hour,
	})
	defer rl.Stop()

	// Max out user1
	rl.RecordFailure("192.168.1.1", "user1")
	rl.RecordFailure("192.168.1.1", "user1")

	// user1 should be blocked
	allowed, _ := rl.Allow("192.168.1.1", "user1")
	if allowed {
		t.Error("user1 should be blocked")
	}

	// user2 should still be allowed
	allowed, _ = rl.Allow("192.168.1.1", "user2")
	if !allowed {
		t.Error("user2 should be allowed")
	}
}

// TestPasswordValidation tests password policy enforcement.
func TestPasswordValidation_MinLength(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"short", true},         // 5 chars
		{"eleven1234", true},    // 10 chars
		{"elevenchar1", true},   // 11 chars
		{"twelvechar12", false}, // 12 chars
		{"verylongpassword", false},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			_, err := HashPassword(tt.password, 4) // Low cost for faster tests
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

// TestSecurityHeaders tests that security headers are set correctly.
func TestSecurityHeaders(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, expected := range headers {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("Header %s = %q, want %q", header, got, expected)
		}
	}

	// CSP should be present
	if csp := rr.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("Content-Security-Policy header should be set")
	}

	// Permissions-Policy should be present
	if pp := rr.Header().Get("Permissions-Policy"); pp == "" {
		t.Error("Permissions-Policy header should be set")
	}
}

// TestSecurityHeadersWithAnalytics tests that analytics URL is added to CSP when set in context.
func TestSecurityHeadersWithAnalytics(t *testing.T) {
	router := gin.New()
	// Simulate AnalyticsContextMiddleware setting the script URL
	router.Use(func(c *gin.Context) {
		c.Set(AnalyticsScriptURLContextKey, "https://analytics.example.com/js/script.js")
		c.Next()
	})
	router.Use(SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header should be set")
	}

	// Verify analytics origin in script-src
	if !strings.Contains(csp, "script-src") || !strings.Contains(csp, "https://analytics.example.com") {
		t.Errorf("CSP script-src should include analytics origin, got: %s", csp)
	}

	// Verify analytics origin in connect-src (for API calls to analytics endpoint)
	if !strings.Contains(csp, "connect-src 'self' https://analytics.example.com") {
		t.Errorf("CSP connect-src should include analytics origin for API calls, got: %s", csp)
	}
}

// TestExtractOrigin tests URL origin extraction for CSP.
func TestExtractOrigin(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://plausible.io/js/script.js", "https://plausible.io"},
		{"https://analytics.example.com/js/script.outbound-links.js", "https://analytics.example.com"},
		{"http://localhost:3000/script.js", "http://localhost:3000"},
		{"", ""},
		{"invalid", "https://invalid"},
	}

	for _, tc := range tests {
		got := extractOrigin(tc.url)
		if got != tc.expected {
			t.Errorf("extractOrigin(%q) = %q, want %q", tc.url, got, tc.expected)
		}
	}
}

// TestHSTSHeader tests HSTS header is only set for HTTPS.
func TestHSTSHeader(t *testing.T) {
	router := gin.New()
	router.Use(StrictTransportSecurityMiddleware(31536000))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// HTTP request - should not have HSTS
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if hsts := rr.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Error("HSTS should not be set for HTTP requests")
	}

	// HTTPS request (via X-Forwarded-Proto) - should have HSTS
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if hsts := rr.Header().Get("Strict-Transport-Security"); hsts == "" {
		t.Error("HSTS should be set for HTTPS requests")
	}
}

// TestUsernameValidation tests username format validation.
func TestUsernameValidation(t *testing.T) {
	tests := []struct {
		username string
		valid    bool
	}{
		{"ab", false},        // Too short
		{"abc", true},        // Minimum
		{"user123", true},    // Alphanumeric
		{"user_name", true},  // With underscore
		{"user-name", true},  // With hyphen
		{"user.name", false}, // Dot not allowed
		{"user@name", false}, // @ not allowed
		{"user name", false}, // Space not allowed
		{"a", false},         // Too short
		{"abcdefghij" + "abcdefghij" + "abcdefghij" + "abcdefghij" + "abcdefghij" + "abcdefghij" + "abcde", false}, // 65 chars, too long
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			result := usernamePattern.MatchString(tt.username)
			if result != tt.valid {
				t.Errorf("username %q validation = %v, want %v", tt.username, result, tt.valid)
			}
		})
	}
}

// TestEmailValidation tests email format validation.
func TestEmailValidation(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"user.name@example.com", true},
		{"user+tag@example.com", true},
		{"user@sub.example.com", true},
		{"invalid", false},
		{"@example.com", false},
		{"user@", false},
		{"user@.com", false},
		{"user@example", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := emailPattern.MatchString(tt.email)
			if result != tt.valid {
				t.Errorf("email %q validation = %v, want %v", tt.email, result, tt.valid)
			}
		})
	}
}
