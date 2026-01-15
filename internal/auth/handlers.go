package auth

import (
	"errors"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

// setupMutex serializes setup requests to prevent race conditions.
var setupMutex sync.Mutex

// isLocalPath validates that a redirect path is local to prevent open redirect attacks.
// Returns true if the path is safe for redirect (local path only).
func isLocalPath(path string) bool {
	if path == "" {
		return false
	}

	// Must start with /
	if !strings.HasPrefix(path, "/") {
		return false
	}

	// Reject protocol-relative URLs (//evil.com)
	if strings.HasPrefix(path, "//") {
		return false
	}

	// Reject URLs with schemes
	if strings.Contains(path, "://") {
		return false
	}

	// Reject paths with backslashes (potential bypass attempts)
	if strings.Contains(path, "\\") {
		return false
	}

	return true
}

// sanitizeRedirectPath returns a safe redirect path, defaulting to "/" if invalid.
func sanitizeRedirectPath(path string) string {
	if isLocalPath(path) {
		return path
	}
	return "/"
}

// AuthController handles authentication-related HTTP endpoints.
type AuthController struct {
	service        *Service
	sessionManager *SessionManager
	templates      *template.Template
	config         config.Auth
	rateLimiter    *RateLimiter
}

// NewAuthController creates a new authentication controller.
func NewAuthController(service *Service, sessionManager *SessionManager, templatesPath string, cfg config.Auth) (*AuthController, error) {
	// Parse auth templates
	pattern := filepath.Join(templatesPath, "auth", "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		// Templates might not exist yet, create controller without them
		tmpl = nil
	}

	// Initialize rate limiter with configuration
	rateLimiter := NewRateLimiter(RateLimitConfig{
		MaxAttempts:     cfg.MaxLoginAttempts,
		WindowDuration:  cfg.RateLimitWindow,
		LockoutDuration: cfg.LockoutDuration,
	})

	return &AuthController{
		service:        service,
		sessionManager: sessionManager,
		templates:      tmpl,
		config:         cfg,
		rateLimiter:    rateLimiter,
	}, nil
}

// RegisterRoutes registers authentication routes on the router.
func (ac *AuthController) RegisterRoutes(router *gin.Engine) {
	router.GET("/login", ac.LoginPage)
	router.POST("/login", ac.Login)
	router.POST("/logout", ac.Logout)
	router.GET("/logout", ac.Logout) // Support GET for simple logout links
	router.GET("/setup", ac.SetupPage)
	router.POST("/setup", ac.Setup)
}

// Stop cleans up resources (rate limiter background goroutine).
func (ac *AuthController) Stop() {
	if ac.rateLimiter != nil {
		ac.rateLimiter.Stop()
	}
}

// LoginPage renders the login form.
func (ac *AuthController) LoginPage(c *gin.Context) {
	// If already authenticated, redirect to home
	if ac.sessionManager != nil && ac.sessionManager.IsAuthenticated(c.Request) {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Sanitize redirect path to prevent open redirect attacks
	next := sanitizeRedirectPath(c.Query("next"))

	// Check if setup is needed
	hasUsers, _ := ac.service.HasUsers()
	if !hasUsers {
		c.Redirect(http.StatusFound, "/setup")
		return
	}

	ac.renderTemplate(c, "login.html", gin.H{
		"Title":     "Login",
		"Next":      next,
		"CSRFToken": GetCSRFToken(c),
		"Error":     c.Query("error"),
	})
}

// Login handles the login form submission.
func (ac *AuthController) Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	// Sanitize redirect path to prevent open redirect attacks
	next := sanitizeRedirectPath(c.PostForm("next"))
	clientIP := c.ClientIP()

	// Check rate limiting before attempting authentication
	if ac.rateLimiter != nil {
		allowed, retryAfter := ac.rateLimiter.Allow(clientIP, username)
		if !allowed {
			c.Header("Retry-After", retryAfter.String())
			ac.renderTemplate(c, "login.html", gin.H{
				"Title":      "Login",
				"Next":       next,
				"Username":   username,
				"CSRFToken":  GetCSRFToken(c),
				"Error":      "Too many login attempts. Please try again later.",
				"RetryAfter": retryAfter.String(),
			})
			return
		}
	}

	// Authenticate user
	user, err := ac.service.Authenticate(username, password)
	if err != nil {
		// Record failed attempt for rate limiting
		if ac.rateLimiter != nil {
			ac.rateLimiter.RecordFailure(clientIP, username)
		}

		errorMsg := "Invalid username or password"
		if errors.Is(err, ErrAccountLocked) {
			errorMsg = "Account is locked. Please try again later."
		}

		ac.renderTemplate(c, "login.html", gin.H{
			"Title":     "Login",
			"Next":      next,
			"Username":  username,
			"CSRFToken": GetCSRFToken(c),
			"Error":     errorMsg,
		})
		return
	}

	// Record successful login (clears rate limit tracking)
	if ac.rateLimiter != nil {
		ac.rateLimiter.RecordSuccess(clientIP, username)
	}

	// Create session
	if ac.sessionManager != nil {
		if err := ac.sessionManager.CreateSession(c.Request, user); err != nil {
			ac.renderTemplate(c, "login.html", gin.H{
				"Title":     "Login",
				"Next":      next,
				"Username":  username,
				"CSRFToken": GetCSRFToken(c),
				"Error":     "Failed to create session",
			})
			return
		}
	}

	c.Redirect(http.StatusFound, next)
}

// Logout destroys the session and redirects to login.
func (ac *AuthController) Logout(c *gin.Context) {
	if ac.sessionManager != nil {
		_ = ac.sessionManager.DestroySession(c.Request)
	}
	c.Redirect(http.StatusFound, "/login")
}

// SetupPage renders the initial admin setup form.
func (ac *AuthController) SetupPage(c *gin.Context) {
	// Only show setup if no users exist
	hasUsers, err := ac.service.HasUsers()
	if err != nil {
		ac.renderTemplate(c, "setup.html", gin.H{
			"Title":     "Initial Setup",
			"CSRFToken": GetCSRFToken(c),
			"Error":     "Database error. Please try again.",
		})
		return
	}
	if hasUsers {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// Check for error query parameter (e.g., from CSRF failure redirect)
	errorMsg := c.Query("error")

	ac.renderTemplate(c, "setup.html", gin.H{
		"Title":     "Initial Setup",
		"CSRFToken": GetCSRFToken(c),
		"Error":     errorMsg,
	})
}

// Setup handles the initial admin user creation.
// Uses a mutex to prevent race conditions where concurrent requests both pass HasUsers() check.
func (ac *AuthController) Setup(c *gin.Context) {
	// Serialize setup requests to prevent race conditions
	setupMutex.Lock()
	defer setupMutex.Unlock()

	// Only allow setup if no users exist (check while holding mutex)
	hasUsers, err := ac.service.HasUsers()
	if err != nil {
		ac.renderTemplate(c, "setup.html", gin.H{
			"Title":     "Initial Setup",
			"CSRFToken": GetCSRFToken(c),
			"Error":     "Database error. Please try again.",
		})
		return
	}
	if hasUsers {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	username := c.PostForm("username")
	email := c.PostForm("email")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	// Validate passwords match
	if password != confirmPassword {
		ac.renderTemplate(c, "setup.html", gin.H{
			"Title":     "Initial Setup",
			"Username":  username,
			"Email":     email,
			"CSRFToken": GetCSRFToken(c),
			"Error":     "Passwords do not match",
		})
		return
	}

	// Create admin user
	user, err := ac.service.CreateUser(username, email, password, entities.UserRoleAdmin)
	if err != nil {
		errorMsg := "Failed to create user"
		switch {
		case errors.Is(err, ErrPasswordTooShort):
			errorMsg = "Password must be at least 12 characters"
		case errors.Is(err, ErrPasswordTooLong):
			errorMsg = "Password exceeds maximum length of 72 characters"
		case errors.Is(err, ErrUsernameRequired):
			errorMsg = "Username is required"
		case errors.Is(err, ErrUsernameInvalid):
			errorMsg = "Username must be 3-64 characters, alphanumeric with underscore/hyphen only"
		case errors.Is(err, ErrEmailRequired):
			errorMsg = "Email is required"
		case errors.Is(err, ErrEmailInvalid):
			errorMsg = "Invalid email format"
		case errors.Is(err, ErrUserExists):
			// Another request won the race, redirect to login
			c.Redirect(http.StatusFound, "/login")
			return
		}

		ac.renderTemplate(c, "setup.html", gin.H{
			"Title":     "Initial Setup",
			"Username":  username,
			"Email":     email,
			"CSRFToken": GetCSRFToken(c),
			"Error":     errorMsg,
		})
		return
	}

	// Create session for new user
	if ac.sessionManager != nil {
		_ = ac.sessionManager.CreateSession(c.Request, user)
	}

	c.Redirect(http.StatusFound, "/")
}

// renderTemplate renders an auth template or falls back to JSON.
func (ac *AuthController) renderTemplate(c *gin.Context, name string, data gin.H) {
	if ac.templates == nil {
		c.JSON(http.StatusOK, data)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := ac.templates.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
	}
}

// APITokenController handles API token management endpoints.
type APITokenController struct {
	service *Service
}

// NewAPITokenController creates a new API token controller.
func NewAPITokenController(service *Service) *APITokenController {
	return &APITokenController{service: service}
}

// GenerateToken creates a new API token for the authenticated user.
func (tc *APITokenController) GenerateToken(c *gin.Context) {
	userID := GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	token, err := tc.service.GenerateToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"message": "Store this token securely - it will not be shown again",
	})
}

// RevokeToken revokes the API token for the authenticated user.
func (tc *APITokenController) RevokeToken(c *gin.Context) {
	userID := GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	if err := tc.service.RevokeToken(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "token revoked"})
}
