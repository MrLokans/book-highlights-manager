// Package tokenstore provides secure storage for OAuth tokens using AES-256-GCM encryption.
package tokenstore

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mrlokans/assistant/internal/crypto"
	"github.com/mrlokans/assistant/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	// EnvEncryptionKey is the environment variable for the encryption key
	EnvEncryptionKey = "TOKEN_ENCRYPTION_KEY"

	// DefaultKeyFileName is the default name for the key file
	DefaultKeyFileName = ".assistant-token-key"
)

// TokenStore provides secure storage for OAuth tokens
type TokenStore struct {
	db        *gorm.DB
	encryptor *crypto.Encryptor
}

// Config holds configuration for the token store
type Config struct {
	// DatabasePath is the path to the SQLite database file
	DatabasePath string

	// EncryptionKey is the base64-encoded 32-byte encryption key
	// If empty, will try to load from environment or key file
	EncryptionKey string

	// KeyFilePath is the path to the encryption key file
	// If empty, defaults to ~/.assistant-token-key
	KeyFilePath string
}

// New creates a new TokenStore with the given configuration
func New(cfg Config) (*TokenStore, error) {
	// Resolve encryption key
	key, err := resolveEncryptionKey(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve encryption key: %w", err)
	}

	// Create encryptor
	encryptor, err := crypto.NewEncryptorFromBase64(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	// Open database
	db, err := gorm.Open(sqlite.Open(cfg.DatabasePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&entities.OAuthToken{}); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	return &TokenStore{
		db:        db,
		encryptor: encryptor,
	}, nil
}

// resolveEncryptionKey determines the encryption key from various sources
func resolveEncryptionKey(cfg Config) (string, error) {
	// Priority 1: Explicitly provided key
	if cfg.EncryptionKey != "" {
		return cfg.EncryptionKey, nil
	}

	// Priority 2: Environment variable
	if envKey := os.Getenv(EnvEncryptionKey); envKey != "" {
		return envKey, nil
	}

	// Priority 3: Key file
	keyFilePath := cfg.KeyFilePath
	if keyFilePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		keyFilePath = filepath.Join(homeDir, DefaultKeyFileName)
	}

	// Try to read existing key file
	if data, err := os.ReadFile(keyFilePath); err == nil {
		return string(data), nil
	}

	// Generate new key and save it
	newKey, err := crypto.GenerateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate encryption key: %w", err)
	}

	// Save key to file with restricted permissions
	if err := os.WriteFile(keyFilePath, []byte(newKey), 0600); err != nil {
		return "", fmt.Errorf("failed to save encryption key to %s: %w", keyFilePath, err)
	}

	fmt.Printf("🔑 Generated new encryption key and saved to %s\n", keyFilePath)
	return newKey, nil
}

// SaveToken saves an OAuth token with encryption
func (s *TokenStore) SaveToken(token *entities.DecryptedToken) error {
	// Encrypt sensitive fields
	encAccessToken, err := s.encryptor.Encrypt(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encRefreshToken, err := s.encryptor.Encrypt(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// Create or update the token record
	dbToken := &entities.OAuthToken{
		Provider:     token.Provider,
		AccountID:    token.AccountID,
		AccessToken:  encAccessToken,
		RefreshToken: encRefreshToken,
		TokenType:    token.TokenType,
		ExpiresAt:    token.ExpiresAt,
		Scope:        token.Scope,
	}

	// Upsert: update if exists, create if not
	result := s.db.Where("provider = ? AND account_id = ?", token.Provider, token.AccountID).
		Assign(map[string]interface{}{
			"access_token":  encAccessToken,
			"refresh_token": encRefreshToken,
			"token_type":    token.TokenType,
			"expires_at":    token.ExpiresAt,
			"scope":         token.Scope,
			"updated_at":    time.Now(),
		}).
		FirstOrCreate(dbToken)

	if result.Error != nil {
		return fmt.Errorf("failed to save token: %w", result.Error)
	}

	return nil
}

// GetToken retrieves and decrypts an OAuth token
func (s *TokenStore) GetToken(provider entities.OAuthProvider, accountID string) (*entities.DecryptedToken, error) {
	var dbToken entities.OAuthToken
	result := s.db.Where("provider = ? AND account_id = ?", provider, accountID).First(&dbToken)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get token: %w", result.Error)
	}

	return s.decryptToken(&dbToken)
}

// GetTokenByProvider retrieves the first token for a provider (useful for single-user scenarios)
func (s *TokenStore) GetTokenByProvider(provider entities.OAuthProvider) (*entities.DecryptedToken, error) {
	var dbToken entities.OAuthToken
	result := s.db.Where("provider = ?", provider).Order("updated_at DESC").First(&dbToken)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get token: %w", result.Error)
	}

	return s.decryptToken(&dbToken)
}

// ListTokens returns all tokens for a provider (without decrypting)
func (s *TokenStore) ListTokens(provider entities.OAuthProvider) ([]entities.OAuthToken, error) {
	var tokens []entities.OAuthToken
	result := s.db.Where("provider = ?", provider).Find(&tokens)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", result.Error)
	}
	return tokens, nil
}

// DeleteToken removes a token from storage
func (s *TokenStore) DeleteToken(provider entities.OAuthProvider, accountID string) error {
	result := s.db.Where("provider = ? AND account_id = ?", provider, accountID).
		Delete(&entities.OAuthToken{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete token: %w", result.Error)
	}
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp for a token
func (s *TokenStore) UpdateLastUsed(provider entities.OAuthProvider, accountID string) error {
	now := time.Now()
	result := s.db.Model(&entities.OAuthToken{}).
		Where("provider = ? AND account_id = ?", provider, accountID).
		Update("last_used_at", now)
	if result.Error != nil {
		return fmt.Errorf("failed to update last used: %w", result.Error)
	}
	return nil
}

// UpdateTokenAfterRefresh updates the access token and refresh token after a refresh
func (s *TokenStore) UpdateTokenAfterRefresh(provider entities.OAuthProvider, accountID string, newAccessToken string, newRefreshToken string, expiresAt *time.Time) error {
	encAccessToken, err := s.encryptor.Encrypt(newAccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	updates := map[string]interface{}{
		"access_token":      encAccessToken,
		"expires_at":        expiresAt,
		"last_refreshed_at": time.Now(),
	}

	// Only update refresh token if a new one was provided
	if newRefreshToken != "" {
		encRefreshToken, err := s.encryptor.Encrypt(newRefreshToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
		updates["refresh_token"] = encRefreshToken
	}

	result := s.db.Model(&entities.OAuthToken{}).
		Where("provider = ? AND account_id = ?", provider, accountID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update token: %w", result.Error)
	}

	return nil
}

// decryptToken decrypts the sensitive fields of a token
func (s *TokenStore) decryptToken(dbToken *entities.OAuthToken) (*entities.DecryptedToken, error) {
	accessToken, err := s.encryptor.Decrypt(dbToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	refreshToken, err := s.encryptor.Decrypt(dbToken.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	return &entities.DecryptedToken{
		Provider:     dbToken.Provider,
		AccountID:    dbToken.AccountID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    dbToken.TokenType,
		ExpiresAt:    dbToken.ExpiresAt,
		Scope:        dbToken.Scope,
	}, nil
}

// Close closes the database connection
func (s *TokenStore) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// GetKeyFilePath returns the path to the key file being used
func GetKeyFilePath(customPath string) string {
	if customPath != "" {
		return customPath
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DefaultKeyFileName
	}
	return filepath.Join(homeDir, DefaultKeyFileName)
}

// GenerateNewKey generates a new encryption key and returns it as base64
func GenerateNewKey() (string, error) {
	keyBytes, err := crypto.GenerateKeyBytes()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(keyBytes), nil
}
