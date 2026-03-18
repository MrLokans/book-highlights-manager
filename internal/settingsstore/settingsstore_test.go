package settingsstore

import (
	"testing"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/database/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (SettingsDB, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test_settings.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	repo := settings.NewRepository(db.DB)
	cleanup := func() {
		db.Close()
	}
	return repo, cleanup
}

func TestNew(t *testing.T) {
	t.Run("creates settings store with database", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		store := New(repo)

		assert.NotNil(t, store)
		assert.Equal(t, repo, store.db)
	})
}
