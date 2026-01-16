package settingsstore

import (
	"os"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObsidianSyncEnabled(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Default should be false
	assert.False(t, store.GetObsidianSyncEnabled())
	assert.Equal(t, "default", store.GetObsidianSyncEnabledSource())

	// Set via database
	err := store.SetObsidianSyncEnabled(true)
	require.NoError(t, err)

	assert.True(t, store.GetObsidianSyncEnabled())
	assert.Equal(t, "database", store.GetObsidianSyncEnabledSource())

	// Clear and verify fallback
	err = db.DeleteSetting(entities.SettingKeyObsidianSyncEnabled)
	require.NoError(t, err)

	assert.False(t, store.GetObsidianSyncEnabled())
	assert.Equal(t, "default", store.GetObsidianSyncEnabledSource())
}

func TestObsidianSyncEnabledWithEnv(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set environment variable
	os.Setenv("OBSIDIAN_SYNC_ENABLED", "true")
	defer os.Unsetenv("OBSIDIAN_SYNC_ENABLED")

	// Should read from env
	assert.True(t, store.GetObsidianSyncEnabled())
	assert.Equal(t, "environment", store.GetObsidianSyncEnabledSource())

	// Database should override env
	err := store.SetObsidianSyncEnabled(false)
	require.NoError(t, err)

	assert.False(t, store.GetObsidianSyncEnabled())
	assert.Equal(t, "database", store.GetObsidianSyncEnabledSource())
}

func TestObsidianSyncExportDir(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Clear any existing env vars for this test
	originalEnv := os.Getenv("OBSIDIAN_EXPORT_DIR")
	originalLegacyEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
	os.Unsetenv("OBSIDIAN_EXPORT_DIR")
	os.Unsetenv("OBSIDIAN_VAULT_DIR")
	defer func() {
		if originalEnv != "" {
			os.Setenv("OBSIDIAN_EXPORT_DIR", originalEnv)
		}
		if originalLegacyEnv != "" {
			os.Setenv("OBSIDIAN_VAULT_DIR", originalLegacyEnv)
		}
	}()

	// Default should be empty
	assert.Empty(t, store.GetObsidianSyncExportDir())
	assert.Equal(t, "default", store.GetObsidianSyncExportDirSource())

	// Set via database
	err := store.SetObsidianSyncExportDir("/test/export")
	require.NoError(t, err)

	assert.Equal(t, "/test/export", store.GetObsidianSyncExportDir())
	assert.Equal(t, "database", store.GetObsidianSyncExportDirSource())
}

func TestObsidianSyncExportDirWithEnv(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set new environment variable
	os.Setenv("OBSIDIAN_EXPORT_DIR", "/env/export")
	defer os.Unsetenv("OBSIDIAN_EXPORT_DIR")

	// Should read from env
	assert.Equal(t, "/env/export", store.GetObsidianSyncExportDir())
	assert.Equal(t, "environment", store.GetObsidianSyncExportDirSource())

	// Database should override env
	err := store.SetObsidianSyncExportDir("/db/export")
	require.NoError(t, err)

	assert.Equal(t, "/db/export", store.GetObsidianSyncExportDir())
	assert.Equal(t, "database", store.GetObsidianSyncExportDirSource())
}

func TestObsidianSyncExportDirLegacyEnv(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Clear new env var, set legacy env var
	os.Unsetenv("OBSIDIAN_EXPORT_DIR")
	os.Setenv("OBSIDIAN_VAULT_DIR", "/legacy/vault")
	defer os.Unsetenv("OBSIDIAN_VAULT_DIR")

	// Should read from legacy env
	assert.Equal(t, "/legacy/vault", store.GetObsidianSyncExportDir())
	assert.Equal(t, "environment", store.GetObsidianSyncExportDirSource())
}

func TestObsidianSyncSchedule(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Default should be hourly
	assert.Equal(t, "0 * * * *", store.GetObsidianSyncSchedule())
	assert.Equal(t, "default", store.GetObsidianSyncScheduleSource())

	// Set via database
	err := store.SetObsidianSyncSchedule("*/15 * * * *")
	require.NoError(t, err)

	assert.Equal(t, "*/15 * * * *", store.GetObsidianSyncSchedule())
	assert.Equal(t, "database", store.GetObsidianSyncScheduleSource())
}

func TestObsidianSyncScheduleWithEnv(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set environment variable
	os.Setenv("OBSIDIAN_SYNC_SCHEDULE", "0 0 * * *")
	defer os.Unsetenv("OBSIDIAN_SYNC_SCHEDULE")

	// Should read from env
	assert.Equal(t, "0 0 * * *", store.GetObsidianSyncSchedule())
	assert.Equal(t, "environment", store.GetObsidianSyncScheduleSource())

	// Database should override env
	err := store.SetObsidianSyncSchedule("*/30 * * * *")
	require.NoError(t, err)

	assert.Equal(t, "*/30 * * * *", store.GetObsidianSyncSchedule())
	assert.Equal(t, "database", store.GetObsidianSyncScheduleSource())
}

func TestObsidianSyncConfig(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Set all values
	require.NoError(t, store.SetObsidianSyncEnabled(true))
	require.NoError(t, store.SetObsidianSyncExportDir("/my/export"))
	require.NoError(t, store.SetObsidianSyncSchedule("0 */6 * * *"))

	config := store.GetObsidianSyncConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, "/my/export", config.ExportDir)
	assert.Equal(t, "0 */6 * * *", config.Schedule)

	// Test info version
	info := store.GetObsidianSyncConfigInfo()
	assert.True(t, info.Enabled)
	assert.Equal(t, "database", info.EnabledSource)
	assert.Equal(t, "/my/export", info.ExportDir)
	assert.Equal(t, "database", info.ExportDirSource)
	assert.Equal(t, "0 */6 * * *", info.Schedule)
	assert.Equal(t, "database", info.ScheduleSource)
}

func TestObsidianSyncStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Initially no status
	status := store.GetObsidianSyncStatus()
	assert.Nil(t, status.LastSyncAt)
	assert.Empty(t, status.Status)
	assert.Empty(t, status.Message)

	// Set success status
	err := store.SetObsidianSyncStatus("success", "Exported 10 books, 50 highlights")
	require.NoError(t, err)

	status = store.GetObsidianSyncStatus()
	assert.NotNil(t, status.LastSyncAt)
	assert.Equal(t, "success", status.Status)
	assert.Equal(t, "Exported 10 books, 50 highlights", status.Message)

	// Verify timestamp is recent
	assert.True(t, time.Since(*status.LastSyncAt) < time.Minute)

	// Set failed status
	err = store.SetObsidianSyncStatus("failed", "Vault directory not found")
	require.NoError(t, err)

	status = store.GetObsidianSyncStatus()
	assert.Equal(t, "failed", status.Status)
	assert.Equal(t, "Vault directory not found", status.Message)
}

func TestClearObsidianSyncSettings(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Clear any existing env vars for this test
	originalExportDir := os.Getenv("OBSIDIAN_EXPORT_DIR")
	originalVaultDir := os.Getenv("OBSIDIAN_VAULT_DIR")
	originalSchedule := os.Getenv("OBSIDIAN_SYNC_SCHEDULE")
	originalEnabled := os.Getenv("OBSIDIAN_SYNC_ENABLED")
	os.Unsetenv("OBSIDIAN_EXPORT_DIR")
	os.Unsetenv("OBSIDIAN_VAULT_DIR")
	os.Unsetenv("OBSIDIAN_SYNC_SCHEDULE")
	os.Unsetenv("OBSIDIAN_SYNC_ENABLED")
	defer func() {
		if originalExportDir != "" {
			os.Setenv("OBSIDIAN_EXPORT_DIR", originalExportDir)
		}
		if originalVaultDir != "" {
			os.Setenv("OBSIDIAN_VAULT_DIR", originalVaultDir)
		}
		if originalSchedule != "" {
			os.Setenv("OBSIDIAN_SYNC_SCHEDULE", originalSchedule)
		}
		if originalEnabled != "" {
			os.Setenv("OBSIDIAN_SYNC_ENABLED", originalEnabled)
		}
	}()

	// Set all values
	require.NoError(t, store.SetObsidianSyncEnabled(true))
	require.NoError(t, store.SetObsidianSyncExportDir("/my/export"))
	require.NoError(t, store.SetObsidianSyncSchedule("0 */6 * * *"))

	// Clear all
	err := store.ClearObsidianSyncSettings()
	require.NoError(t, err)

	// Should fall back to defaults
	assert.False(t, store.GetObsidianSyncEnabled())
	assert.Equal(t, "default", store.GetObsidianSyncEnabledSource())
	assert.Empty(t, store.GetObsidianSyncExportDir())
	assert.Equal(t, "default", store.GetObsidianSyncExportDirSource())
	assert.Equal(t, "0 * * * *", store.GetObsidianSyncSchedule())
	assert.Equal(t, "default", store.GetObsidianSyncScheduleSource())
}

func TestValidateCronSchedule(t *testing.T) {
	tests := []struct {
		schedule string
		valid    bool
	}{
		{"0 * * * *", true},       // Every hour
		{"*/15 * * * *", true},    // Every 15 minutes
		{"0 0 * * *", true},       // Daily at midnight
		{"0 0 * * 0", true},       // Weekly on Sunday
		{"0 */6 * * *", true},     // Every 6 hours
		{"invalid", false},        // Invalid
		{"* * * *", false},        // Missing field
		{"60 * * * *", false},     // Invalid minute
		{"0 25 * * *", false},     // Invalid hour
	}

	for _, tt := range tests {
		t.Run(tt.schedule, func(t *testing.T) {
			err := ValidateCronSchedule(tt.schedule)
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestGetCronDescription(t *testing.T) {
	tests := []struct {
		schedule    string
		description string
	}{
		{"0 * * * *", "Every hour at :00"},
		{"*/15 * * * *", "Every 15 minutes"},
		{"*/30 * * * *", "Every 30 minutes"},
		{"0 */6 * * *", "Every 6 hours"},
		{"0 0 * * *", "Daily at midnight"},
		{"0 0 * * 0", "Weekly on Sunday at midnight"},
		{"5 4 * * *", "Custom schedule: 5 4 * * *"},
	}

	for _, tt := range tests {
		t.Run(tt.schedule, func(t *testing.T) {
			desc := GetCronDescription(tt.schedule)
			assert.Equal(t, tt.description, desc)
		})
	}
}

func TestGetNextRunTime(t *testing.T) {
	// Test valid schedule
	next, err := GetNextRunTime("0 * * * *")
	require.NoError(t, err)
	assert.NotNil(t, next)
	assert.True(t, next.After(time.Now()))

	// Test invalid schedule
	_, err = GetNextRunTime("invalid")
	assert.Error(t, err)
}
