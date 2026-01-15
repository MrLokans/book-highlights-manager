package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

// Context keys for user data
const (
	ContextKeyUserID   = "auth_user_id"
	ContextKeyUsername = "auth_username"
	ContextKeyRole     = "auth_role"
	ContextKeyAuthType = "auth_type" // "session", "bearer", or "none"
)

// AuthType indicates how the user was authenticated
type AuthType string

const (
	AuthTypeNone    AuthType = "none"
	AuthTypeSession AuthType = "session"
	AuthTypeBearer  AuthType = "bearer"
)

// DefaultUserID is used when authentication is disabled
const DefaultUserID = uint(0)

// Middleware handles authentication for HTTP requests.
type Middleware struct {
	service        *Service
	sessionManager *SessionManager
	config         config.Auth
	publicPaths    map[string]bool
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(service *Service, sessionManager *SessionManager, cfg config.Auth) *Middleware {
	publicPaths := map[string]bool{
		"/health":         true,
		"/ping":           true,
		"/login":          true,
		"/setup":          true,
		"/static":         true, // Static files prefix
		"/favicon.ico":    true,
	}

	return &Middleware{
		service:        service,
		sessionManager: sessionManager,
		config:         cfg,
		publicPaths:    publicPaths,
	}
}

// Handler returns a Gin middleware handler that authenticates requests.
func (m *Middleware) Handler() gin.HandlerFunc {
	// If auth is disabled, inject default user
	if m.config.Mode == config.AuthModeNone {
		return m.noAuthHandler()
	}

	return m.authHandler()
}

// noAuthHandler injects DefaultUserID for all requests when auth is disabled.
func (m *Middleware) noAuthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextKeyUserID, DefaultUserID)
		c.Set(ContextKeyAuthType, AuthTypeNone)
		c.Next()
	}
}

// authHandler handles authentication when auth is enabled.
func (m *Middleware) authHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if path is public
		if m.isPublicPath(c.Request.URL.Path) {
			c.Set(ContextKeyUserID, DefaultUserID)
			c.Set(ContextKeyAuthType, AuthTypeNone)
			c.Next()
			return
		}

		// Try Bearer token first (for API clients)
		if user := m.tryBearerAuth(c); user != nil {
			m.setUserContext(c, user, AuthTypeBearer)
			c.Next()
			return
		}

		// Try session auth (for web UI)
		if user := m.trySessionAuth(c); user != nil {
			m.setUserContext(c, user, AuthTypeSession)
			c.Next()
			return
		}

		// Not authenticated - check if this is an API request
		if m.isAPIRequest(c) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		// Web request - redirect to login
		c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
		c.Abort()
	}
}

// tryBearerAuth attempts to authenticate using Bearer token.
func (m *Middleware) tryBearerAuth(c *gin.Context) *entities.User {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil
	}

	// Extract token from "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil
	}

	token := parts[1]
	user, err := m.service.ValidateToken(token)
	if err != nil {
		return nil
	}

	return user
}

// trySessionAuth attempts to authenticate using session cookie.
func (m *Middleware) trySessionAuth(c *gin.Context) *entities.User {
	if m.sessionManager == nil {
		return nil
	}

	userID := m.sessionManager.GetUserID(c.Request)
	if userID == 0 {
		return nil
	}

	user, err := m.service.GetUserByID(userID)
	if err != nil {
		return nil
	}

	return user
}

// setUserContext stores user information in the Gin context.
func (m *Middleware) setUserContext(c *gin.Context, user *entities.User, authType AuthType) {
	c.Set(ContextKeyUserID, user.ID)
	c.Set(ContextKeyUsername, user.Username)
	c.Set(ContextKeyRole, user.Role)
	c.Set(ContextKeyAuthType, authType)
}

// isPublicPath checks if a path should be accessible without authentication.
func (m *Middleware) isPublicPath(path string) bool {
	// Exact match
	if m.publicPaths[path] {
		return true
	}

	// Prefix match for static files
	if strings.HasPrefix(path, "/static/") {
		return true
	}

	return false
}

// isAPIRequest determines if this is an API request vs web browser request.
func (m *Middleware) isAPIRequest(c *gin.Context) bool {
	// Check for API path prefix
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		return true
	}

	// Check Accept header
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}

	// Check for Bearer token attempt (even if invalid)
	if c.GetHeader("Authorization") != "" {
		return true
	}

	return false
}

// RequireAuth returns a middleware that requires authentication.
// Use this for routes that must be protected even if they're not in the default list.
func (m *Middleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 && m.config.Mode == config.AuthModeLocal {
			if m.isAPIRequest(c) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "authentication required",
				})
			} else {
				c.Redirect(http.StatusFound, "/login?next="+c.Request.URL.Path)
				c.Abort()
			}
			return
		}
		c.Next()
	}
}

// RequireRole returns a middleware that requires a specific role.
func (m *Middleware) RequireRole(roles ...entities.UserRole) gin.HandlerFunc {
	roleSet := make(map[entities.UserRole]bool)
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		// Skip role check if auth is disabled
		if m.config.Mode == config.AuthModeNone {
			c.Next()
			return
		}

		role := GetUserRole(c)
		if !roleSet[role] {
			if m.isAPIRequest(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "insufficient permissions",
				})
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
		c.Next()
	}
}

// Helper functions to extract auth data from Gin context

// GetUserID retrieves the authenticated user's ID from the context.
// Returns DefaultUserID (0) if not authenticated or auth is disabled.
func GetUserID(c *gin.Context) uint {
	if id, exists := c.Get(ContextKeyUserID); exists {
		if userID, ok := id.(uint); ok {
			return userID
		}
	}
	return DefaultUserID
}

// GetUsername retrieves the authenticated user's username from the context.
func GetUsername(c *gin.Context) string {
	if name, exists := c.Get(ContextKeyUsername); exists {
		if username, ok := name.(string); ok {
			return username
		}
	}
	return ""
}

// GetUserRole retrieves the authenticated user's role from the context.
func GetUserRole(c *gin.Context) entities.UserRole {
	if r, exists := c.Get(ContextKeyRole); exists {
		if role, ok := r.(entities.UserRole); ok {
			return role
		}
	}
	return ""
}

// GetAuthType retrieves the authentication method used.
func GetAuthType(c *gin.Context) AuthType {
	if t, exists := c.Get(ContextKeyAuthType); exists {
		if authType, ok := t.(AuthType); ok {
			return authType
		}
	}
	return AuthTypeNone
}

// IsAuthenticated returns true if the request is authenticated.
func IsAuthenticated(c *gin.Context) bool {
	return GetUserID(c) != 0 || GetAuthType(c) == AuthTypeNone
}
