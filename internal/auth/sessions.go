package auth

import (
	"database/sql"
	"encoding/gob"
	"net/http"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
)

// Session data keys
const (
	SessionKeyUserID   = "user_id"
	SessionKeyUsername = "username"
	SessionKeyRole     = "role"
	SessionKeyLoginAt  = "login_at"
)

func init() {
	// Register types that will be stored in sessions
	gob.Register(entities.UserRole(""))
	gob.Register(time.Time{})
}

// SessionManager wraps scs.SessionManager with application-specific methods.
type SessionManager struct {
	*scs.SessionManager
}

// NewSessionManager creates a configured session manager.
// The sqlDB parameter should be the underlying *sql.DB from GORM.
func NewSessionManager(sqlDB *sql.DB, cfg config.Auth) (*SessionManager, error) {
	// Create sessions table if it doesn't exist
	_, err := sqlDB.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		data BLOB NOT NULL,
		expiry REAL NOT NULL
	);
	CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);`)
	if err != nil {
		return nil, err
	}

	sm := scs.New()

	// Configure session store (SQLite)
	store := sqlite3store.New(sqlDB)
	sm.Store = store

	// Configure session lifetime
	sm.Lifetime = cfg.SessionLifetime
	sm.IdleTimeout = cfg.SessionLifetime / 2 // Half of lifetime for inactivity

	// Configure cookie security
	sm.Cookie.Name = "session"
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = cfg.SecureCookies
	sm.Cookie.SameSite = http.SameSiteStrictMode // Strict for better security
	sm.Cookie.Path = "/"

	return &SessionManager{SessionManager: sm}, nil
}

// CreateSession creates a new session for a user after successful authentication.
// This should be called after password verification.
func (sm *SessionManager) CreateSession(r *http.Request, user *entities.User) error {
	// Renew token to prevent session fixation
	if err := sm.RenewToken(r.Context()); err != nil {
		return err
	}

	// Store user ID as int to match GetInt() retrieval
	sm.Put(r.Context(), SessionKeyUserID, int(user.ID))
	sm.Put(r.Context(), SessionKeyUsername, user.Username)
	sm.Put(r.Context(), SessionKeyRole, user.Role)
	sm.Put(r.Context(), SessionKeyLoginAt, time.Now())

	return nil
}

// DestroySession removes all session data and invalidates the session.
func (sm *SessionManager) DestroySession(r *http.Request) error {
	return sm.Destroy(r.Context())
}

// GetUserID retrieves the user ID from the session.
// Returns 0 if not authenticated.
func (sm *SessionManager) GetUserID(r *http.Request) uint {
	return uint(sm.GetInt(r.Context(), SessionKeyUserID))
}

// GetUsername retrieves the username from the session.
func (sm *SessionManager) GetUsername(r *http.Request) string {
	return sm.GetString(r.Context(), SessionKeyUsername)
}

// GetUserRole retrieves the user role from the session.
func (sm *SessionManager) GetUserRole(r *http.Request) entities.UserRole {
	role, ok := sm.Get(r.Context(), SessionKeyRole).(entities.UserRole)
	if !ok {
		return ""
	}
	return role
}

// IsAuthenticated returns true if the request has a valid session.
func (sm *SessionManager) IsAuthenticated(r *http.Request) bool {
	return sm.GetUserID(r) != 0
}

// SessionData holds the session information for a request.
type SessionData struct {
	UserID   uint
	Username string
	Role     entities.UserRole
	LoginAt  time.Time
}

// GetSessionData retrieves all session data at once.
func (sm *SessionManager) GetSessionData(r *http.Request) *SessionData {
	userID := sm.GetUserID(r)
	if userID == 0 {
		return nil
	}

	loginAt, _ := sm.Get(r.Context(), SessionKeyLoginAt).(time.Time)

	return &SessionData{
		UserID:   userID,
		Username: sm.GetUsername(r),
		Role:     sm.GetUserRole(r),
		LoginAt:  loginAt,
	}
}
