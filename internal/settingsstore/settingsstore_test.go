package settingsstore

import (
	"os"
	"strings"
	"testing"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*database.Database, func()) {
	t.Helper()
	dbPath := "./test_settings_" + strings.ReplaceAll(t.Name(), "/", "_") + ".db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, cleanup
}

func TestNew(t *testing.T) {
	t.Run("creates settings store with database", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		store := New(db)

		assert.NotNil(t, store)
		assert.Equal(t, db, store.db)
	})
}

func TestGetMarkdownExportPath(t *testing.T) {
	t.Run("returns database value when set", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Clear any env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Unsetenv("OBSIDIAN_VAULT_DIR")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		// Set value in database
		db.SetSetting(entities.SettingKeyMarkdownExportPath, "/custom/path")

		store := New(db)
		path := store.GetMarkdownExportPath()

		assert.Equal(t, "/custom/path", path)
	})

	t.Run("returns environment variable when database not set", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Set env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Setenv("OBSIDIAN_VAULT_DIR", "/env/vault/path")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		store := New(db)
		path := store.GetMarkdownExportPath()

		assert.Equal(t, "/env/vault/path", path)
	})

	t.Run("returns current working directory when nothing else set", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Clear env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Unsetenv("OBSIDIAN_VAULT_DIR")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		store := New(db)
		path := store.GetMarkdownExportPath()

		// Should return current working directory
		cwd, _ := os.Getwd()
		assert.Equal(t, cwd, path)
	})

	t.Run("database takes priority over environment variable", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Set both env var and database value
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Setenv("OBSIDIAN_VAULT_DIR", "/env/path")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		db.SetSetting(entities.SettingKeyMarkdownExportPath, "/db/path")

		store := New(db)
		path := store.GetMarkdownExportPath()

		assert.Equal(t, "/db/path", path)
	})
}

func TestSetMarkdownExportPath(t *testing.T) {
	t.Run("saves path to database", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		store := New(db)

		err := store.SetMarkdownExportPath("/new/export/path")
		require.NoError(t, err)

		// Verify it was saved
		setting, err := db.GetSetting(entities.SettingKeyMarkdownExportPath)
		require.NoError(t, err)
		assert.Equal(t, "/new/export/path", setting.Value)
	})

	t.Run("updates existing path", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		store := New(db)

		err := store.SetMarkdownExportPath("/first/path")
		require.NoError(t, err)

		err = store.SetMarkdownExportPath("/second/path")
		require.NoError(t, err)

		// Verify it was updated
		setting, err := db.GetSetting(entities.SettingKeyMarkdownExportPath)
		require.NoError(t, err)
		assert.Equal(t, "/second/path", setting.Value)
	})
}

func TestGetMarkdownExportPathSource(t *testing.T) {
	t.Run("returns database when database value set", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Clear env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Unsetenv("OBSIDIAN_VAULT_DIR")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		db.SetSetting(entities.SettingKeyMarkdownExportPath, "/db/path")

		store := New(db)
		source := store.GetMarkdownExportPathSource()

		assert.Equal(t, "database", source)
	})

	t.Run("returns environment when env var set", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Set env var only
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Setenv("OBSIDIAN_VAULT_DIR", "/env/path")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		store := New(db)
		source := store.GetMarkdownExportPathSource()

		assert.Equal(t, "environment", source)
	})

	t.Run("returns default when nothing set", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Clear env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Unsetenv("OBSIDIAN_VAULT_DIR")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		store := New(db)
		source := store.GetMarkdownExportPathSource()

		assert.Equal(t, "default", source)
	})

	t.Run("database takes priority in source detection", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Set both
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Setenv("OBSIDIAN_VAULT_DIR", "/env/path")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		db.SetSetting(entities.SettingKeyMarkdownExportPath, "/db/path")

		store := New(db)
		source := store.GetMarkdownExportPathSource()

		assert.Equal(t, "database", source)
	})
}

func TestGetMarkdownExportPathInfo(t *testing.T) {
	t.Run("returns path and source together", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Clear env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Unsetenv("OBSIDIAN_VAULT_DIR")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		db.SetSetting(entities.SettingKeyMarkdownExportPath, "/info/path")

		store := New(db)
		info := store.GetMarkdownExportPathInfo()

		assert.Equal(t, "/info/path", info.Path)
		assert.Equal(t, "database", info.Source)
	})

	t.Run("returns env path and source", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Setenv("OBSIDIAN_VAULT_DIR", "/env/info/path")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		store := New(db)
		info := store.GetMarkdownExportPathInfo()

		assert.Equal(t, "/env/info/path", info.Path)
		assert.Equal(t, "environment", info.Source)
	})

	t.Run("returns default path and source", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Unsetenv("OBSIDIAN_VAULT_DIR")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		store := New(db)
		info := store.GetMarkdownExportPathInfo()

		cwd, _ := os.Getwd()
		assert.Equal(t, cwd, info.Path)
		assert.Equal(t, "default", info.Source)
	})
}

func TestClearMarkdownExportPath(t *testing.T) {
	t.Run("removes database setting", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Clear env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Unsetenv("OBSIDIAN_VAULT_DIR")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		// Set a value
		db.SetSetting(entities.SettingKeyMarkdownExportPath, "/to/clear")

		store := New(db)

		// Verify it exists
		assert.Equal(t, "database", store.GetMarkdownExportPathSource())

		// Clear it
		err := store.ClearMarkdownExportPath()
		require.NoError(t, err)

		// Should now use default
		assert.Equal(t, "default", store.GetMarkdownExportPathSource())
	})

	t.Run("does not error when nothing to clear", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		store := New(db)

		err := store.ClearMarkdownExportPath()
		assert.NoError(t, err)
	})

	t.Run("falls back to env after clear", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		// Set env var
		originalEnv := os.Getenv("OBSIDIAN_VAULT_DIR")
		os.Setenv("OBSIDIAN_VAULT_DIR", "/fallback/env")
		defer os.Setenv("OBSIDIAN_VAULT_DIR", originalEnv)

		// Set database value (takes priority)
		db.SetSetting(entities.SettingKeyMarkdownExportPath, "/db/value")

		store := New(db)

		// Verify database is used
		assert.Equal(t, "/db/value", store.GetMarkdownExportPath())
		assert.Equal(t, "database", store.GetMarkdownExportPathSource())

		// Clear database setting
		err := store.ClearMarkdownExportPath()
		require.NoError(t, err)

		// Should now use env
		assert.Equal(t, "/fallback/env", store.GetMarkdownExportPath())
		assert.Equal(t, "environment", store.GetMarkdownExportPathSource())
	})
}

func TestExportPathInfo(t *testing.T) {
	t.Run("struct has correct JSON tags", func(t *testing.T) {
		info := ExportPathInfo{
			Path:   "/test/path",
			Source: "database",
		}

		assert.Equal(t, "/test/path", info.Path)
		assert.Equal(t, "database", info.Source)
	})
}
