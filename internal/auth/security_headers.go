package auth

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds security headers to all responses.
// These headers help protect against common web vulnerabilities.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Enable XSS filter in browsers (legacy, but still useful)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer policy - don't leak URLs to external sites
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy - restrict resource loading
		// Note: 'unsafe-inline' for style-src needed for HTMX attributes
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'; "+
				"form-action 'self'")

		// Permissions Policy - disable unnecessary browser features
		c.Header("Permissions-Policy",
			"accelerometer=(), "+
				"camera=(), "+
				"geolocation=(), "+
				"gyroscope=(), "+
				"magnetometer=(), "+
				"microphone=(), "+
				"payment=(), "+
				"usb=()")

		c.Next()
	}
}

// StrictTransportSecurityMiddleware adds HSTS header for HTTPS-only access.
// Only enable this when serving over HTTPS, as it will break HTTP access.
func StrictTransportSecurityMiddleware(maxAge int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only set HSTS if the request came over HTTPS
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			// max-age in seconds (31536000 = 1 year)
			// includeSubDomains protects all subdomains
			c.Header("Strict-Transport-Security",
				"max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
