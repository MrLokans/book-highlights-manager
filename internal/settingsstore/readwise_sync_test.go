package settingsstore

import (
	"os"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadwiseSyncEnabled(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Default should be false
	assert.False(t, store.GetReadwiseSyncEnabled())
	assert.Equal(t, "default", store.GetReadwiseSyncEnabledSource())

	// Set via database
	err := store.SetReadwiseSyncEnabled(true)
	require.NoError(t, err)

	assert.True(t, store.GetReadwiseSyncEnabled())
	assert.Equal(t, "database", store.GetReadwiseSyncEnabledSource())

	// Clear and verify fallback
	err = db.DeleteSetting(entities.SettingKeyReadwiseSyncEnabled)
	require.NoError(t, err)

	assert.False(t, store.GetReadwiseSyncEnabled())
	assert.Equal(t, "default", store.GetReadwiseSyncEnabledSource())
}

func TestReadwiseSyncEnabledWithEnv(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set environment variable
	os.Setenv("READWISE_SYNC_ENABLED", "true")
	defer os.Unsetenv("READWISE_SYNC_ENABLED")

	// Should read from env
	assert.True(t, store.GetReadwiseSyncEnabled())
	assert.Equal(t, "environment", store.GetReadwiseSyncEnabledSource())

	// Database should override env
	err := store.SetReadwiseSyncEnabled(false)
	require.NoError(t, err)

	assert.False(t, store.GetReadwiseSyncEnabled())
	assert.Equal(t, "database", store.GetReadwiseSyncEnabledSource())
}

func TestReadwiseSyncToken(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Clear any existing env vars
	originalToken := os.Getenv("READWISE_TOKEN")
	os.Unsetenv("READWISE_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("READWISE_TOKEN", originalToken)
		}
	}()

	// Default should be empty
	assert.Empty(t, store.GetReadwiseSyncToken())
	assert.Equal(t, "default", store.GetReadwiseSyncTokenSource())
	assert.False(t, store.HasReadwiseSyncToken())

	// Set via database
	err := store.SetReadwiseSyncToken("test-token-12345")
	require.NoError(t, err)

	assert.Equal(t, "test-token-12345", store.GetReadwiseSyncToken())
	assert.Equal(t, "database", store.GetReadwiseSyncTokenSource())
	assert.True(t, store.HasReadwiseSyncToken())
}

func TestReadwiseSyncTokenWithEnv(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set environment variable
	os.Setenv("READWISE_TOKEN", "env-token-abc")
	defer os.Unsetenv("READWISE_TOKEN")

	// Should read from env
	assert.Equal(t, "env-token-abc", store.GetReadwiseSyncToken())
	assert.Equal(t, "environment", store.GetReadwiseSyncTokenSource())
	assert.True(t, store.HasReadwiseSyncToken())

	// Database should override env
	err := store.SetReadwiseSyncToken("db-token-xyz")
	require.NoError(t, err)

	assert.Equal(t, "db-token-xyz", store.GetReadwiseSyncToken())
	assert.Equal(t, "database", store.GetReadwiseSyncTokenSource())
}

func TestReadwiseSyncSchedule(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Default should be every 6 hours
	assert.Equal(t, "0 */6 * * *", store.GetReadwiseSyncSchedule())
	assert.Equal(t, "default", store.GetReadwiseSyncScheduleSource())

	// Set via database
	err := store.SetReadwiseSyncSchedule("*/15 * * * *")
	require.NoError(t, err)

	assert.Equal(t, "*/15 * * * *", store.GetReadwiseSyncSchedule())
	assert.Equal(t, "database", store.GetReadwiseSyncScheduleSource())
}

func TestReadwiseSyncScheduleWithEnv(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set environment variable
	os.Setenv("READWISE_SYNC_SCHEDULE", "0 0 * * *")
	defer os.Unsetenv("READWISE_SYNC_SCHEDULE")

	// Should read from env
	assert.Equal(t, "0 0 * * *", store.GetReadwiseSyncSchedule())
	assert.Equal(t, "environment", store.GetReadwiseSyncScheduleSource())

	// Database should override env
	err := store.SetReadwiseSyncSchedule("*/30 * * * *")
	require.NoError(t, err)

	assert.Equal(t, "*/30 * * * *", store.GetReadwiseSyncSchedule())
	assert.Equal(t, "database", store.GetReadwiseSyncScheduleSource())
}

func TestReadwiseSyncConfig(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set all values
	require.NoError(t, store.SetReadwiseSyncEnabled(true))
	require.NoError(t, store.SetReadwiseSyncToken("test-token-12345"))
	require.NoError(t, store.SetReadwiseSyncSchedule("0 */6 * * *"))

	config := store.GetReadwiseSyncConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, "test-token-12345", config.Token)
	assert.Equal(t, "0 */6 * * *", config.Schedule)

	// Test info version (with masked token)
	info := store.GetReadwiseSyncConfigInfo()
	assert.True(t, info.Enabled)
	assert.Equal(t, "database", info.EnabledSource)
	assert.Equal(t, "test****2345", info.Token) // Masked
	assert.Equal(t, "database", info.TokenSource)
	assert.True(t, info.HasToken)
	assert.Equal(t, "0 */6 * * *", info.Schedule)
	assert.Equal(t, "database", info.ScheduleSource)
}

func TestReadwiseSyncStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Initially no status
	status := store.GetReadwiseSyncStatus()
	assert.Nil(t, status.LastSyncAt)
	assert.Empty(t, status.Status)
	assert.Empty(t, status.Message)
	assert.Zero(t, status.HighlightsSynced)

	// Set success status
	err := store.SetReadwiseSyncStatus("success", "Imported 5 books with 100 highlights", 100)
	require.NoError(t, err)

	status = store.GetReadwiseSyncStatus()
	assert.NotNil(t, status.LastSyncAt)
	assert.Equal(t, "success", status.Status)
	assert.Equal(t, "Imported 5 books with 100 highlights", status.Message)
	assert.Equal(t, 100, status.HighlightsSynced)

	// Verify timestamp is recent
	assert.True(t, time.Since(*status.LastSyncAt) < time.Minute)

	// Set failed status
	err = store.SetReadwiseSyncStatus("failed", "Invalid token", 0)
	require.NoError(t, err)

	status = store.GetReadwiseSyncStatus()
	assert.Equal(t, "failed", status.Status)
	assert.Equal(t, "Invalid token", status.Message)
	assert.Zero(t, status.HighlightsSynced)
}

func TestReadwiseSyncLastAt(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Initially nil
	assert.Nil(t, store.GetReadwiseSyncLastAt())

	// After setting status, should have a timestamp
	err := store.SetReadwiseSyncStatus("success", "test", 50)
	require.NoError(t, err)

	lastAt := store.GetReadwiseSyncLastAt()
	assert.NotNil(t, lastAt)
	assert.True(t, time.Since(*lastAt) < time.Minute)
}

func TestClearReadwiseSyncSettings(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Clear any existing env vars
	originalToken := os.Getenv("READWISE_TOKEN")
	originalSchedule := os.Getenv("READWISE_SYNC_SCHEDULE")
	originalEnabled := os.Getenv("READWISE_SYNC_ENABLED")
	os.Unsetenv("READWISE_TOKEN")
	os.Unsetenv("READWISE_SYNC_SCHEDULE")
	os.Unsetenv("READWISE_SYNC_ENABLED")
	defer func() {
		if originalToken != "" {
			os.Setenv("READWISE_TOKEN", originalToken)
		}
		if originalSchedule != "" {
			os.Setenv("READWISE_SYNC_SCHEDULE", originalSchedule)
		}
		if originalEnabled != "" {
			os.Setenv("READWISE_SYNC_ENABLED", originalEnabled)
		}
	}()

	// Set all values
	require.NoError(t, store.SetReadwiseSyncEnabled(true))
	require.NoError(t, store.SetReadwiseSyncToken("test-token"))
	require.NoError(t, store.SetReadwiseSyncSchedule("0 */6 * * *"))

	// Clear all
	err := store.ClearReadwiseSyncSettings()
	require.NoError(t, err)

	// Should fall back to defaults
	assert.False(t, store.GetReadwiseSyncEnabled())
	assert.Equal(t, "default", store.GetReadwiseSyncEnabledSource())
	assert.Empty(t, store.GetReadwiseSyncToken())
	assert.Equal(t, "default", store.GetReadwiseSyncTokenSource())
	assert.Equal(t, "0 */6 * * *", store.GetReadwiseSyncSchedule())
	assert.Equal(t, "default", store.GetReadwiseSyncScheduleSource())
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		token    string
		expected string
	}{
		{"", ""},
		{"1234", "****"},
		{"12345678", "****"},
		{"123456789", "1234****6789"},
		{"test-token-12345", "test****2345"},
		{"abcdefghijklmnop", "abcd****mnop"},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			result := maskToken(tt.token)
			assert.Equal(t, tt.expected, result)
		})
	}
}
