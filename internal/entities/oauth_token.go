package entities

import (
	"time"

	"gorm.io/gorm"
)

// OAuthProvider represents the OAuth provider type
type OAuthProvider string

const (
	OAuthProviderDropbox OAuthProvider = "dropbox"
	OAuthProviderGoogle  OAuthProvider = "google"
)

// OAuthToken stores encrypted OAuth tokens for external services
type OAuthToken struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Provider identifies the OAuth service (e.g., "dropbox", "google")
	Provider OAuthProvider `gorm:"type:varchar(50);not null;uniqueIndex:idx_provider_account" json:"provider"`

	// AccountID is the user's account identifier on the provider (e.g., email or account ID)
	AccountID string `gorm:"type:varchar(255);not null;uniqueIndex:idx_provider_account" json:"account_id"`

	// AccessToken is the encrypted OAuth access token
	// Stored as base64-encoded AES-256-GCM ciphertext
	AccessToken string `gorm:"type:text;not null" json:"-"`

	// RefreshToken is the encrypted OAuth refresh token
	// Stored as base64-encoded AES-256-GCM ciphertext
	RefreshToken string `gorm:"type:text" json:"-"`

	// TokenType is typically "Bearer"
	TokenType string `gorm:"type:varchar(50);default:Bearer" json:"token_type"`

	// ExpiresAt is when the access token expires (nullable for non-expiring tokens)
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Scope contains the OAuth scopes granted
	Scope string `gorm:"type:text" json:"scope,omitempty"`

	// LastUsedAt tracks when the token was last used
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`

	// LastRefreshedAt tracks when the token was last refreshed
	LastRefreshedAt *time.Time `json:"last_refreshed_at,omitempty"`
}

// TableName specifies the table name for GORM
func (OAuthToken) TableName() string {
	return "oauth_tokens"
}

// IsExpired checks if the access token has expired
func (t *OAuthToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	// Consider expired if less than 5 minutes remaining
	return time.Now().Add(5 * time.Minute).After(*t.ExpiresAt)
}

// IsExpiringSoon checks if the token expires within the given duration
func (t *OAuthToken) IsExpiringSoon(within time.Duration) bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().Add(within).After(*t.ExpiresAt)
}

// DecryptedToken holds the decrypted token values for use in memory
// This is never stored directly in the database
type DecryptedToken struct {
	Provider     OAuthProvider
	AccountID    string
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    *time.Time
	Scope        string
}
