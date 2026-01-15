package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

var _ *gorm.DB // Ensure gorm is used for sqlite driver

func setupSessionManager(t *testing.T) (*SessionManager, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// AutoMigrate User for session tests
	if err := db.AutoMigrate(&entities.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

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

	cfg := config.Auth{
		Mode:            config.AuthModeLocal,
		SessionLifetime: 24 * time.Hour,
		SecureCookies:   false,
	}

	sm, err := NewSessionManager(sqlDB, cfg)
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	return sm, db
}

func TestNewSessionManager(t *testing.T) {
	sm, _ := setupSessionManager(t)

	if sm == nil {
		t.Fatal("session manager should not be nil")
	}

	if sm.SessionManager == nil {
		t.Fatal("inner session manager should not be nil")
	}

	// Verify cookie configuration
	if sm.Cookie.Name != "session" {
		t.Errorf("Expected cookie name 'session', got '%s'", sm.Cookie.Name)
	}
	if !sm.Cookie.HttpOnly {
		t.Error("Cookie should be HttpOnly")
	}
	if sm.Cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected SameSiteStrictMode, got %v", sm.Cookie.SameSite)
	}
}

func TestSessionManager_CreateAndRetrieveSession(t *testing.T) {
	sm, _ := setupSessionManager(t)

	user := &entities.User{
		ID:       123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     entities.UserRoleAdmin,
	}

	// Create a test request with session middleware
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// Wrap handler with session middleware
	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create session
		err := sm.CreateSession(r, user)
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// Verify session data is available
		userID := sm.GetUserID(r)
		if userID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, userID)
		}

		username := sm.GetUsername(r)
		if username != user.Username {
			t.Errorf("Expected username '%s', got '%s'", user.Username, username)
		}

		role := sm.GetUserRole(r)
		if role != user.Role {
			t.Errorf("Expected role '%s', got '%s'", user.Role, role)
		}

		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestSessionManager_IsAuthenticated(t *testing.T) {
	sm, _ := setupSessionManager(t)

	user := &entities.User{
		ID:       456,
		Username: "authuser",
		Email:    "auth@example.com",
		Role:     entities.UserRoleViewer,
	}

	// Test unauthenticated request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Before login, should not be authenticated
		if sm.IsAuthenticated(r) {
			t.Error("Should not be authenticated before login")
		}

		// Create session
		if err := sm.CreateSession(r, user); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// After login, should be authenticated
		if !sm.IsAuthenticated(r) {
			t.Error("Should be authenticated after login")
		}

		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)
}

func TestSessionManager_DestroySession(t *testing.T) {
	sm, _ := setupSessionManager(t)

	user := &entities.User{
		ID:       789,
		Username: "destroyuser",
		Email:    "destroy@example.com",
		Role:     entities.UserRoleEditor,
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create session
		if err := sm.CreateSession(r, user); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// Verify session exists
		if !sm.IsAuthenticated(r) {
			t.Error("Should be authenticated after login")
		}

		// Destroy session
		if err := sm.DestroySession(r); err != nil {
			t.Fatalf("failed to destroy session: %v", err)
		}

		// After destroy, should not be authenticated
		if sm.IsAuthenticated(r) {
			t.Error("Should not be authenticated after session destroy")
		}

		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)
}

func TestSessionManager_GetSessionData(t *testing.T) {
	sm, _ := setupSessionManager(t)

	user := &entities.User{
		ID:       999,
		Username: "datauser",
		Email:    "data@example.com",
		Role:     entities.UserRoleAdmin,
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Before login, GetSessionData should return nil
		data := sm.GetSessionData(r)
		if data != nil {
			t.Error("GetSessionData should return nil for unauthenticated request")
		}

		// Create session
		if err := sm.CreateSession(r, user); err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		// After login, GetSessionData should return data
		data = sm.GetSessionData(r)
		if data == nil {
			t.Fatal("GetSessionData should not return nil after login")
		}

		if data.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, data.UserID)
		}
		if data.Username != user.Username {
			t.Errorf("Expected username '%s', got '%s'", user.Username, data.Username)
		}
		if data.Role != user.Role {
			t.Errorf("Expected role '%s', got '%s'", user.Role, data.Role)
		}
		if data.LoginAt.IsZero() {
			t.Error("LoginAt should not be zero")
		}

		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)
}

func TestSessionManager_EmptyRole(t *testing.T) {
	sm, _ := setupSessionManager(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Without session, GetUserRole should return empty string
		role := sm.GetUserRole(r)
		if role != "" {
			t.Errorf("Expected empty role, got '%s'", role)
		}

		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)
}

func TestSessionManager_SecureCookieConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get SQL DB: %v", err)
	}

	// Create sessions table
	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		data BLOB NOT NULL,
		expiry REAL NOT NULL
	);
	CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);`)
	if err != nil {
		t.Fatalf("failed to create sessions table: %v", err)
	}

	// Test with secure cookies enabled
	cfg := config.Auth{
		Mode:            config.AuthModeLocal,
		SessionLifetime: 24 * time.Hour,
		SecureCookies:   true,
	}

	sm, err := NewSessionManager(sqlDB, cfg)
	if err != nil {
		t.Fatalf("failed to create session manager: %v", err)
	}

	if !sm.Cookie.Secure {
		t.Error("Cookie.Secure should be true when SecureCookies is enabled")
	}
}
