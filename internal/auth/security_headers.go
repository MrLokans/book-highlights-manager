package auth

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// AnalyticsScriptURLContextKey is the Gin context key for the analytics script URL.
// Set by AnalyticsContextMiddleware, read by SecurityHeadersMiddleware.
const AnalyticsScriptURLContextKey = "analytics_script_url"

// SecurityHeadersMiddleware adds security headers to all responses.
// If analytics is configured, reads script URL from Gin context (set by AnalyticsContextMiddleware).
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

		// Build script-src and connect-src with optional analytics domain
		// Note: 'unsafe-eval' needed for HTMX's hx-on attributes
		scriptSrc := "'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com"
		connectSrc := "'self'"
		if analyticsURL, exists := c.Get(AnalyticsScriptURLContextKey); exists {
			if urlStr, ok := analyticsURL.(string); ok && urlStr != "" {
				if origin := extractOrigin(urlStr); origin != "" {
					scriptSrc += " " + origin
					connectSrc += " " + origin
				}
			}
		}

		// Build form-action with explicit host to handle reverse proxy scenarios
		// 'self' can fail when behind proxies like cloudflared
		formAction := "'self'"
		if host := c.Request.Host; host != "" {
			// Include both HTTP and HTTPS variants to be safe
			formAction = "'self' https://" + host
		}

		// Content Security Policy - restrict resource loading
		// Note: 'unsafe-inline' for style-src needed for HTMX attributes
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src "+scriptSrc+"; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self'; "+
				"connect-src "+connectSrc+"; "+
				"frame-ancestors 'none'; "+
				"form-action "+formAction)

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

// extractOrigin extracts the origin (scheme + host) from a URL for CSP
func extractOrigin(rawURL string) string {
	// Handle URLs without scheme
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}

	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}

	return scheme + "://" + parsed.Host
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
