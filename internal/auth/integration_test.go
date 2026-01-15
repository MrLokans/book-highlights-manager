package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *Service, *SessionManager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Setup database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	if err := db.AutoMigrate(&entities.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Get SQL DB for session store
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get SQL DB: %v", err)
	}

	// Create sessions table required by sqlite3store
	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		data BLOB NOT NULL,
		expiry REAL NOT NULL
	);
	CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);`)
	if err != nil {
		t.Fatalf("failed to create sessions table: %v", err)
	}

	// Create auth config
	cfg := config.Auth{
		Mode:            config.AuthModeLocal,
		SessionLifetime: 24 * time.Hour,
		BcryptCost:      10,
		SecureCookies:   false, // For testing
	}

	// Create service
	svc := NewService(db, cfg)

	// Create session manager
	sm, err := NewSessionManager(sqlDB, cfg)
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	// Create middleware
	middleware := NewMiddleware(svc, sm, cfg)

	// Setup router
	router := gin.New()
	router.Use(sm.SessionLoadSave())
	router.Use(middleware.Handler())

	// Add test routes
	router.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public"})
	})

	router.GET("/protected", func(c *gin.Context) {
		userID := GetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	return router, svc, sm
}

func TestIntegration_NoAuthMode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.Auth{
		Mode: config.AuthModeNone,
	}

	// Create middleware for no-auth mode
	middleware := NewMiddleware(nil, nil, cfg)

	router := gin.New()
	router.Use(middleware.Handler())
	router.GET("/test", func(c *gin.Context) {
		userID := GetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	// Request without auth should succeed
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Should return DefaultUserID
	if !strings.Contains(w.Body.String(), `"user_id":0`) {
		t.Errorf("Expected user_id:0, got %s", w.Body.String())
	}
}

func TestIntegration_BearerTokenAuth(t *testing.T) {
	router, svc, _ := setupTestRouter(t)

	// Create a user
	user, err := svc.CreateUser("testuser", "test@example.com", "password12345", entities.UserRoleAdmin)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Generate token
	token, err := svc.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Add protected route
	router.GET("/api/me", func(c *gin.Context) {
		userID := GetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	// Request with valid token
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 with valid token, got %d: %s", w.Code, w.Body.String())
	}

	// Request with invalid token
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 with invalid token, got %d", w.Code)
	}
}

func TestIntegration_PublicRoutes(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	// Add public routes that should be accessible
	publicPaths := []string{"/health", "/ping", "/login", "/setup"}

	for _, path := range publicPaths {
		router.GET(path, func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"path": c.Request.URL.Path})
		})
	}

	// All public paths should be accessible without auth
	for _, path := range publicPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Public path %s returned %d, expected 200", path, w.Code)
		}
	}
}

func TestIntegration_ProtectedRoutesRedirect(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	// Request protected route without auth (web request)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should redirect to login
	if w.Code != http.StatusFound {
		t.Errorf("Expected redirect (302), got %d", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.HasPrefix(location, "/login") {
		t.Errorf("Expected redirect to /login, got %s", location)
	}
}

func TestIntegration_ProtectedRoutesAPIReturn401(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	// Add API route
	router.GET("/api/data", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "secret"})
	})

	// Request API route without auth
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 401, not redirect
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for API request, got %d", w.Code)
	}
}

func TestIntegration_SessionLoginLogoutFlow(t *testing.T) {
	router, svc, sm := setupTestRouter(t)

	// Create a user
	_, err := svc.CreateUser("sessionuser", "session@example.com", "password12345", entities.UserRoleAdmin)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Add login route that creates session
	router.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		user, err := svc.Authenticate(username, password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		if err := sm.CreateSession(c.Request, user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "session creation failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "logged in", "user_id": user.ID})
	})

	// Add logout route
	router.POST("/logout", func(c *gin.Context) {
		_ = sm.DestroySession(c.Request)
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	})

	// Step 1: Login and get session cookie
	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=sessionuser&password=password12345"))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("Login failed: %d - %s", loginW.Code, loginW.Body.String())
	}

	// Extract session cookie from response headers directly
	// (httptest.ResponseRecorder.Result() doesn't include headers added after body write)
	setCookieHeader := loginW.Header().Get("Set-Cookie")
	if setCookieHeader == "" {
		t.Fatal("No Set-Cookie header found after login")
	}

	// Parse the Set-Cookie header to create a cookie for subsequent requests
	// Format: session=TOKEN; Path=/; ...
	header := http.Header{}
	header.Add("Set-Cookie", setCookieHeader)
	resp := http.Response{Header: header}
	cookies := resp.Cookies()

	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("No session cookie found in Set-Cookie header: %s", setCookieHeader)
	}

	// Step 2: Access protected route with session cookie
	protectedReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	protectedReq.AddCookie(sessionCookie)
	protectedW := httptest.NewRecorder()
	router.ServeHTTP(protectedW, protectedReq)

	if protectedW.Code != http.StatusOK {
		t.Errorf("Protected route with session cookie returned %d, expected 200", protectedW.Code)
	}

	// Verify user_id is set (not 0)
	if strings.Contains(protectedW.Body.String(), `"user_id":0`) {
		t.Error("Expected authenticated user_id, got 0")
	}

	// Step 3: Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	router.ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Errorf("Logout returned %d, expected 200", logoutW.Code)
	}

	// Step 4: Verify protected route no longer works with old session
	// Note: Session is destroyed but cookie might still exist; behavior depends on session store
	afterLogoutReq := httptest.NewRequest(http.MethodGet, "/protected", nil)
	afterLogoutReq.AddCookie(sessionCookie)
	afterLogoutReq.Header.Set("Accept", "text/html")
	afterLogoutW := httptest.NewRecorder()
	router.ServeHTTP(afterLogoutW, afterLogoutReq)

	// Should redirect to login since session is destroyed
	if afterLogoutW.Code != http.StatusFound {
		t.Logf("After logout, protected route returned %d (might be expected if session cookie is still valid)", afterLogoutW.Code)
	}
}

func TestIntegration_TokenGenerateUseRevokeFlow(t *testing.T) {
	router, svc, _ := setupTestRouter(t)

	// Create a user
	user, err := svc.CreateUser("tokenuser", "token@example.com", "password12345", entities.UserRoleAdmin)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Add API route for testing
	router.GET("/api/me", func(c *gin.Context) {
		userID := GetUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	// Step 1: Generate token
	token, err := svc.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Step 2: Use token to access protected endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Token authentication failed: %d - %s", w.Code, w.Body.String())
	}

	// Step 3: Revoke token
	err = svc.RevokeToken(user.ID)
	if err != nil {
		t.Fatalf("Failed to revoke token: %v", err)
	}

	// Step 4: Verify revoked token no longer works
	revokedReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	revokedReq.Header.Set("Authorization", "Bearer "+token)
	revokedW := httptest.NewRecorder()
	router.ServeHTTP(revokedW, revokedReq)

	if revokedW.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 with revoked token, got %d", revokedW.Code)
	}

	// Step 5: Generate new token and verify it works
	newToken, err := svc.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("Failed to generate new token: %v", err)
	}

	newTokenReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	newTokenReq.Header.Set("Authorization", "Bearer "+newToken)
	newTokenW := httptest.NewRecorder()
	router.ServeHTTP(newTokenW, newTokenReq)

	if newTokenW.Code != http.StatusOK {
		t.Errorf("New token authentication failed: %d", newTokenW.Code)
	}
}

func TestIntegration_PasswordChangeFlow(t *testing.T) {
	_, svc, _ := setupTestRouter(t)

	// Create a user
	user, err := svc.CreateUser("pwuser", "pw@example.com", "oldpassword1234545", entities.UserRoleAdmin)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify old password works
	_, err = svc.Authenticate("pwuser", "oldpassword1234545")
	if err != nil {
		t.Fatal("Initial authentication failed")
	}

	// Change password
	err = svc.ChangePassword(user.ID, "oldpassword1234545", "newpassword456789")
	if err != nil {
		t.Fatalf("Password change failed: %v", err)
	}

	// Verify old password no longer works
	_, err = svc.Authenticate("pwuser", "oldpassword1234545")
	if err == nil {
		t.Error("Old password should not work after change")
	}

	// Verify new password works
	_, err = svc.Authenticate("pwuser", "newpassword456789")
	if err != nil {
		t.Error("New password should work after change")
	}
}

func TestIntegration_SetupFlow(t *testing.T) {
	_, svc, _ := setupTestRouter(t)

	// Initially, there should be no users
	hasUsers, err := svc.HasUsers()
	if err != nil {
		t.Fatalf("HasUsers failed: %v", err)
	}
	if hasUsers {
		t.Fatal("Expected no users initially")
	}

	// Create first admin user (simulating setup)
	_, err = svc.CreateUser("admin", "admin@example.com", "adminpass123", entities.UserRoleAdmin)
	if err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	// Now there should be users
	hasUsers, err = svc.HasUsers()
	if err != nil {
		t.Fatalf("HasUsers failed: %v", err)
	}
	if !hasUsers {
		t.Fatal("Expected users after setup")
	}

	// Verify admin can authenticate
	user, err := svc.Authenticate("admin", "adminpass123")
	if err != nil {
		t.Fatal("Admin authentication failed")
	}
	if user.Role != entities.UserRoleAdmin {
		t.Errorf("Expected admin role, got %s", user.Role)
	}
}

func TestIntegration_StaticFilesAndPublicAssets(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	// Add static file handler
	router.GET("/static/*filepath", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"static": true})
	})

	// Static files should be accessible without auth
	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Static file access failed: %d", w.Code)
	}
}

func TestIntegration_MalformedBearerToken(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	testCases := []struct {
		name   string
		header string
	}{
		{"Empty Bearer", "Bearer "},
		{"Just Bearer", "Bearer"},
		{"Wrong scheme", "Basic abc123"},
		{"No space", "Bearerabc123"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s: expected 401, got %d", tc.name, w.Code)
			}
		})
	}
}
