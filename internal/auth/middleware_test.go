package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupMiddleware(t *testing.T, authMode config.AuthMode) (*Middleware, *Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&entities.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	cfg := config.Auth{
		Mode:            authMode,
		SessionLifetime: 24 * time.Hour,
		SecureCookies:   false,
		BcryptCost:      4, // Low cost for faster tests
	}

	service := NewService(db, cfg)
	middleware := NewMiddleware(service, nil, cfg)

	return middleware, service, db
}

func TestMiddleware_NoAuthMode(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeNone)

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/test", func(c *gin.Context) {
		userID := GetUserID(c)
		authType := GetAuthType(c)
		c.JSON(http.StatusOK, gin.H{
			"user_id":   userID,
			"auth_type": authType,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestMiddleware_PublicPaths(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeLocal)

	publicPaths := []string{
		"/health",
		"/ping",
		"/login",
		"/setup",
		"/static/style.css",
		"/favicon.ico",
	}

	for _, path := range publicPaths {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			router.Use(middleware.Handler())
			router.GET(path, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200 for public path %s, got %d", path, rr.Code)
			}
		})
	}
}

func TestMiddleware_ProtectedPath_RedirectsToLogin(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeLocal)

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("Expected redirect (302), got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/login?next=/protected" {
		t.Errorf("Expected redirect to /login?next=/protected, got %s", location)
	}
}

func TestMiddleware_APIPath_Returns401(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeLocal)

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for API path, got %d", rr.Code)
	}
}

func TestMiddleware_BearerAuth_ValidToken(t *testing.T) {
	middleware, service, _ := setupMiddleware(t, config.AuthModeLocal)

	// Create a user with a token
	user, err := service.CreateUser("testuser", "test@example.com", "password12345", entities.UserRoleAdmin)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	token, err := service.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/api/test", func(c *gin.Context) {
		userID := GetUserID(c)
		authType := GetAuthType(c)
		c.JSON(http.StatusOK, gin.H{
			"user_id":   userID,
			"auth_type": authType,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestMiddleware_BearerAuth_InvalidToken(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeLocal)

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken123")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid token, got %d", rr.Code)
	}
}

func TestMiddleware_BearerAuth_MalformedHeader(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeLocal)

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	testCases := []struct {
		name   string
		header string
	}{
		{"missing bearer prefix", "Token abc123"},
		{"basic auth", "Basic abc123"},
		{"no space", "Bearerabc123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Authorization", tc.header)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Expected 401 for malformed auth header, got %d", rr.Code)
			}
		})
	}
}

func TestMiddleware_RequireAuth(t *testing.T) {
	middleware, service, _ := setupMiddleware(t, config.AuthModeLocal)

	// Create a user with a token
	user, _ := service.CreateUser("authuser", "auth@example.com", "password12345", entities.UserRoleAdmin)
	token, _ := service.GenerateToken(user.ID)

	router := gin.New()
	router.Use(middleware.Handler())

	// Protected route using RequireAuth
	router.GET("/must-auth", middleware.RequireAuth(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test without auth
	req := httptest.NewRequest(http.MethodGet, "/must-auth", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound { // Redirect to login
		t.Errorf("Expected redirect (302) without auth, got %d", rr.Code)
	}

	// Test with auth
	req = httptest.NewRequest(http.MethodGet, "/must-auth", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 with valid auth, got %d", rr.Code)
	}
}

func TestMiddleware_RequireRole(t *testing.T) {
	middleware, service, _ := setupMiddleware(t, config.AuthModeLocal)

	// Create users with different roles
	admin, _ := service.CreateUser("admin", "admin@example.com", "password12345", entities.UserRoleAdmin)
	adminToken, _ := service.GenerateToken(admin.ID)

	viewer, _ := service.CreateUser("viewer", "viewer@example.com", "password12345", entities.UserRoleViewer)
	viewerToken, _ := service.GenerateToken(viewer.ID)

	router := gin.New()
	router.Use(middleware.Handler())

	// Admin-only route
	router.GET("/api/admin", middleware.RequireRole(entities.UserRoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test admin accessing admin route
	req := httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for admin accessing admin route, got %d", rr.Code)
	}

	// Test viewer accessing admin route
	req = httptest.NewRequest(http.MethodGet, "/api/admin", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for viewer accessing admin route, got %d", rr.Code)
	}
}

func TestMiddleware_RequireRole_NoAuthMode(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeNone)

	router := gin.New()
	router.Use(middleware.Handler())

	// Admin-only route, but auth is disabled
	router.GET("/admin", middleware.RequireRole(entities.UserRoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Should pass because auth is disabled
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 when auth is disabled, got %d", rr.Code)
	}
}

func TestGetUserID_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	userID := GetUserID(c)
	if userID != DefaultUserID {
		t.Errorf("Expected default user ID %d, got %d", DefaultUserID, userID)
	}
}

func TestGetUsername_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	username := GetUsername(c)
	if username != "" {
		t.Errorf("Expected empty username, got %s", username)
	}
}

func TestGetUserRole_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	role := GetUserRole(c)
	if role != "" {
		t.Errorf("Expected empty role, got %s", role)
	}
}

func TestGetAuthType_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	authType := GetAuthType(c)
	if authType != AuthTypeNone {
		t.Errorf("Expected AuthTypeNone, got %s", authType)
	}
}

func TestIsAuthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("not authenticated", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(ContextKeyAuthType, AuthTypeNone)

		// When auth type is none, user is considered "authenticated" (auth is disabled)
		if !IsAuthenticated(c) {
			t.Error("Expected IsAuthenticated to return true when auth is disabled")
		}
	})

	t.Run("authenticated with user ID", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(ContextKeyUserID, uint(123))
		c.Set(ContextKeyAuthType, AuthTypeSession)

		if !IsAuthenticated(c) {
			t.Error("Expected IsAuthenticated to return true when user ID is set")
		}
	})
}

func TestMiddleware_AcceptHeader_JSON(t *testing.T) {
	middleware, _, _ := setupMiddleware(t, config.AuthModeLocal)

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Should return 401 instead of redirect for JSON requests
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for JSON request, got %d", rr.Code)
	}
}
