package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

// CSRFTemplateField is the template function name for getting the CSRF token field.
const CSRFTemplateField = "csrfField"

// CSRFTokenHeader is the header name for CSRF token in AJAX requests.
const CSRFTokenHeader = "X-CSRF-Token"

// CSRFMiddleware creates a Gin middleware for CSRF protection.
// It skips CSRF checks for:
// - API routes with valid Bearer token authentication
// - Safe HTTP methods (GET, HEAD, OPTIONS, TRACE)
//
// The authService parameter is used to validate bearer tokens before skipping CSRF.
// If nil, bearer tokens are not validated (less secure, for backward compatibility).
func CSRFMiddleware(secret []byte, secure bool, authService *Service) gin.HandlerFunc {
	csrfProtect := csrf.Protect(
		secret,
		csrf.Secure(secure),
		csrf.HttpOnly(true),
		csrf.SameSite(csrf.SameSiteStrictMode), // Strict mode for better CSRF protection
		csrf.Path("/"),
		csrf.ErrorHandler(http.HandlerFunc(csrfErrorHandler)),
	)

	return func(c *gin.Context) {
		// Skip CSRF for API routes with valid Bearer auth
		if isAPIWithValidBearer(c, authService) {
			c.Next()
			return
		}

		// Apply CSRF protection
		handler := csrfProtect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Store the CSRF token in the context for templates
			c.Set("csrf_token", csrf.Token(r))
			// Update request with CSRF context - session middleware runs after this
			// so session context will be added on top of CSRF context
			c.Request = r
			c.Next()
		}))

		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// csrfErrorHandler handles CSRF validation failures.
func csrfErrorHandler(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"CSRF token invalid or missing"}`))
		return
	}

	// For form submissions, redirect back to the original page with an error
	// This provides a better UX than showing a plain text error
	referer := r.Referer()
	if referer != "" {
		// Add error parameter to the referer URL
		separator := "?"
		if strings.Contains(referer, "?") {
			separator = "&"
		}
		http.Redirect(w, r, referer+separator+"error=Session+expired.+Please+try+again.", http.StatusSeeOther)
		return
	}

	// Fallback to HTML error page if no referer
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Session Expired</title></head>
<body style="font-family: system-ui; max-width: 400px; margin: 100px auto; text-align: center;">
<h1>Session Expired</h1>
<p>Your session has expired or the form submission was invalid.</p>
<p><a href="javascript:history.back()">Go back and try again</a></p>
</body>
</html>`))
}

// isAPIWithValidBearer checks if this is an API request with a valid Bearer token.
// If authService is nil, it only checks for header presence (less secure).
func isAPIWithValidBearer(c *gin.Context, authService *Service) bool {
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return false
	}

	// If no auth service provided, fall back to header-only check (backward compat)
	if authService == nil {
		return true
	}

	// Extract and validate the token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return false
	}

	token := parts[1]
	_, err := authService.ValidateToken(token)
	return err == nil
}

// GetCSRFToken retrieves the CSRF token from the Gin context.
func GetCSRFToken(c *gin.Context) string {
	if token, exists := c.Get("csrf_token"); exists {
		if t, ok := token.(string); ok {
			return t
		}
	}
	return ""
}

// CSRFTokenField returns an HTML hidden input field with the CSRF token.
func CSRFTokenField(c *gin.Context) string {
	token := GetCSRFToken(c)
	if token == "" {
		return ""
	}
	return `<input type="hidden" name="gorilla.csrf.Token" value="` + token + `">`
}
