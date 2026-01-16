package settingsstore

import (
	"os"
	"strings"
	"testing"

	"github.com/mrlokans/assistant/internal/database"
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
