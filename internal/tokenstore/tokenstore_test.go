package tokenstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/crypto"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (*TokenStore, func()) {
	t.Helper()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "tokenstore-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tempDir, "test.db")
	keyPath := filepath.Join(tempDir, "test-key")

	// Generate encryption key
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte(key), 0600)
	require.NoError(t, err)

	store, err := New(Config{
		DatabasePath:  dbPath,
		EncryptionKey: key,
	})
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tempDir)
	}

	return store, cleanup
}

func TestNew(t *testing.T) {
	t.Run("creates store with valid config", func(t *testing.T) {
		store, cleanup := setupTestStore(t)
		defer cleanup()
		assert.NotNil(t, store)
	})

	t.Run("fails with invalid encryption key", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "tokenstore-test-*")
		defer os.RemoveAll(tempDir)

		_, err := New(Config{
			DatabasePath:  filepath.Join(tempDir, "test.db"),
			EncryptionKey: "invalid-key",
		})
		assert.Error(t, err)
	})

	t.Run("generates key file if missing", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "tokenstore-test-*")
		defer os.RemoveAll(tempDir)

		keyPath := filepath.Join(tempDir, "new-key")
		dbPath := filepath.Join(tempDir, "test.db")

		store, err := New(Config{
			DatabasePath: dbPath,
			KeyFilePath:  keyPath,
		})
		require.NoError(t, err)
		defer store.Close()

		// Key file should exist
		_, err = os.Stat(keyPath)
		assert.NoError(t, err)

		// Key file should have restricted permissions
		info, _ := os.Stat(keyPath)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}

func TestSaveAndGetToken(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	t.Run("save and retrieve token", func(t *testing.T) {
		expiresAt := time.Now().Add(4 * time.Hour)
		token := &entities.DecryptedToken{
			Provider:     entities.OAuthProviderDropbox,
			AccountID:    "test@example.com",
			AccessToken:  "sl.access-token-12345",
			RefreshToken: "refresh-token-67890",
			TokenType:    "Bearer",
			ExpiresAt:    &expiresAt,
			Scope:        "files.content.read",
		}

		err := store.SaveToken(token)
		require.NoError(t, err)

		// Retrieve token
		retrieved, err := store.GetToken(entities.OAuthProviderDropbox, "test@example.com")
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, token.Provider, retrieved.Provider)
		assert.Equal(t, token.AccountID, retrieved.AccountID)
		assert.Equal(t, token.AccessToken, retrieved.AccessToken)
		assert.Equal(t, token.RefreshToken, retrieved.RefreshToken)
		assert.Equal(t, token.TokenType, retrieved.TokenType)
		assert.Equal(t, token.Scope, retrieved.Scope)
	})

	t.Run("get non-existent token returns nil", func(t *testing.T) {
		retrieved, err := store.GetToken(entities.OAuthProviderDropbox, "nonexistent@example.com")
		require.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("update existing token", func(t *testing.T) {
		token := &entities.DecryptedToken{
			Provider:     entities.OAuthProviderDropbox,
			AccountID:    "update@example.com",
			AccessToken:  "original-access-token",
			RefreshToken: "original-refresh-token",
			TokenType:    "Bearer",
		}

		err := store.SaveToken(token)
		require.NoError(t, err)

		// Update the token
		token.AccessToken = "updated-access-token"
		err = store.SaveToken(token)
		require.NoError(t, err)

		// Retrieve and verify
		retrieved, err := store.GetToken(entities.OAuthProviderDropbox, "update@example.com")
		require.NoError(t, err)
		assert.Equal(t, "updated-access-token", retrieved.AccessToken)
	})
}

func TestGetTokenByProvider(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Save multiple tokens for same provider
	token1 := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    "user1@example.com",
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
	}
	token2 := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    "user2@example.com",
		AccessToken:  "access-2",
		RefreshToken: "refresh-2",
		TokenType:    "Bearer",
	}

	require.NoError(t, store.SaveToken(token1))
	time.Sleep(10 * time.Millisecond) // Ensure different updated_at
	require.NoError(t, store.SaveToken(token2))

	// Should return most recently updated
	retrieved, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "user2@example.com", retrieved.AccountID)
}

func TestDeleteToken(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	token := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    "delete@example.com",
		AccessToken:  "to-be-deleted",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
	}

	require.NoError(t, store.SaveToken(token))

	// Verify it exists
	retrieved, err := store.GetToken(entities.OAuthProviderDropbox, "delete@example.com")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Delete it
	err = store.DeleteToken(entities.OAuthProviderDropbox, "delete@example.com")
	require.NoError(t, err)

	// Verify it's gone
	retrieved, err = store.GetToken(entities.OAuthProviderDropbox, "delete@example.com")
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestUpdateTokenAfterRefresh(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	originalExpiry := time.Now().Add(1 * time.Hour)
	token := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    "refresh@example.com",
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		ExpiresAt:    &originalExpiry,
	}

	require.NoError(t, store.SaveToken(token))

	// Refresh the token
	newExpiry := time.Now().Add(4 * time.Hour)
	err := store.UpdateTokenAfterRefresh(
		entities.OAuthProviderDropbox,
		"refresh@example.com",
		"new-access-token",
		"", // Dropbox doesn't rotate refresh tokens
		&newExpiry,
	)
	require.NoError(t, err)

	// Verify updates
	retrieved, err := store.GetToken(entities.OAuthProviderDropbox, "refresh@example.com")
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", retrieved.AccessToken)
	assert.Equal(t, "old-refresh-token", retrieved.RefreshToken) // Should be unchanged
}

func TestListTokens(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Save tokens for different providers
	dropboxToken := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    "list@example.com",
		AccessToken:  "dropbox-access",
		RefreshToken: "dropbox-refresh",
		TokenType:    "Bearer",
	}
	googleToken := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderGoogle,
		AccountID:    "list@example.com",
		AccessToken:  "google-access",
		RefreshToken: "google-refresh",
		TokenType:    "Bearer",
	}

	require.NoError(t, store.SaveToken(dropboxToken))
	require.NoError(t, store.SaveToken(googleToken))

	// List Dropbox tokens
	tokens, err := store.ListTokens(entities.OAuthProviderDropbox)
	require.NoError(t, err)
	assert.Len(t, tokens, 1)
	assert.Equal(t, entities.OAuthProviderDropbox, tokens[0].Provider)
}

func TestTokenEncryption(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tokenstore-test-*")
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	key, _ := crypto.GenerateKey()

	store, err := New(Config{
		DatabasePath:  dbPath,
		EncryptionKey: key,
	})
	require.NoError(t, err)

	token := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    "encrypt@example.com",
		AccessToken:  "my-secret-access-token",
		RefreshToken: "my-secret-refresh-token",
		TokenType:    "Bearer",
	}

	require.NoError(t, store.SaveToken(token))
	store.Close()

	// Open database directly and verify tokens are encrypted
	store2, err := New(Config{
		DatabasePath:  dbPath,
		EncryptionKey: key,
	})
	require.NoError(t, err)
	defer store2.Close()

	// Get raw record from DB
	var rawToken entities.OAuthToken
	err = store2.db.Where("provider = ?", entities.OAuthProviderDropbox).First(&rawToken).Error
	require.NoError(t, err)

	// Raw values should NOT be plaintext
	assert.NotEqual(t, "my-secret-access-token", rawToken.AccessToken)
	assert.NotEqual(t, "my-secret-refresh-token", rawToken.RefreshToken)

	// But decrypted values should match
	decrypted, err := store2.GetToken(entities.OAuthProviderDropbox, "encrypt@example.com")
	require.NoError(t, err)
	assert.Equal(t, "my-secret-access-token", decrypted.AccessToken)
	assert.Equal(t, "my-secret-refresh-token", decrypted.RefreshToken)
}

func TestTokenEncryptionWithWrongKey(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "tokenstore-test-*")
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()

	// Save with key1
	store1, err := New(Config{
		DatabasePath:  dbPath,
		EncryptionKey: key1,
	})
	require.NoError(t, err)

	token := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    "wrongkey@example.com",
		AccessToken:  "secret-token",
		RefreshToken: "secret-refresh",
		TokenType:    "Bearer",
	}
	require.NoError(t, store1.SaveToken(token))
	store1.Close()

	// Try to read with key2
	store2, err := New(Config{
		DatabasePath:  dbPath,
		EncryptionKey: key2,
	})
	require.NoError(t, err)
	defer store2.Close()

	// Should fail to decrypt
	_, err = store2.GetToken(entities.OAuthProviderDropbox, "wrongkey@example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decrypt")
}
