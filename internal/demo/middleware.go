package demo

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Middleware blocks write operations in demo mode.
// Read-only operations (GET) are always allowed.
// Certain paths are allowlisted even for non-GET methods (e.g., search).
type Middleware struct {
	enabled bool
}

// NewMiddleware creates a demo mode middleware.
func NewMiddleware(enabled bool) *Middleware {
	return &Middleware{enabled: enabled}
}

// IsEnabled returns whether demo mode is active.
func (m *Middleware) IsEnabled() bool {
	return m.enabled
}

// Handler returns a Gin middleware that blocks write operations.
func (m *Middleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.enabled {
			c.Next()
			return
		}

		// Always allow GET requests (read-only)
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		// Allow HEAD and OPTIONS for CORS/preflight
		if c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// Check if path is in the allowlist for non-GET methods
		if m.isAllowedPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Block the request
		m.respondBlocked(c)
	}
}

// isAllowedPath checks if a path is allowed for write operations in demo mode.
// This is intentionally restrictive - only explicitly allowed paths pass through.
func (m *Middleware) isAllowedPath(path string) bool {
	allowedPaths := []string{
		// Auth endpoints need to work for login flow
		"/login",
		"/logout",
		"/auth/",
	}

	for _, allowed := range allowedPaths {
		if strings.HasPrefix(path, allowed) {
			return true
		}
	}
	return false
}

// respondBlocked sends a 403 response with an appropriate message.
// Supports both JSON API and HTMX responses.
func (m *Middleware) respondBlocked(c *gin.Context) {
	message := "This action is disabled in demo mode"

	// Check if this is an HTMX request
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Reswap", "none")
		c.Header("HX-Trigger", `{"showToast": {"message": "`+message+`", "type": "warning"}}`)
		c.String(http.StatusForbidden, message)
		c.Abort()
		return
	}

	// Check Accept header for JSON preference
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "application/json") {
		c.JSON(http.StatusForbidden, gin.H{
			"error":     message,
			"demo_mode": true,
		})
		c.Abort()
		return
	}

	// Default HTML response
	c.String(http.StatusForbidden, message)
	c.Abort()
}

// ContextKey for storing demo mode state in request context.
const ContextKeyDemoMode = "demo_mode"

// InjectContext middleware adds demo mode flag to context for template rendering.
func (m *Middleware) InjectContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextKeyDemoMode, m.enabled)
		c.Next()
	}
}
