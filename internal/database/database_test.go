package database

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*Database, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := NewDatabase(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
	}
	return db, cleanup
}

func TestNewDatabase(t *testing.T) {
	t.Run("creates database file", func(t *testing.T) {
		dbPath := t.TempDir() + "/init_test.db"
		db, err := NewDatabase(dbPath)
		require.NoError(t, err)
		defer db.Close()

		_, err = os.Stat(dbPath)
		assert.NoError(t, err)
	})

	t.Run("seeds sources on creation", func(t *testing.T) {
		dbPath := t.TempDir() + "/seed_test.db"
		db, err := NewDatabase(dbPath)
		require.NoError(t, err)
		defer db.Close()

		// Verify sources were seeded by querying directly
		var count int64
		db.DB.Table("sources").Count(&count)
		assert.GreaterOrEqual(t, count, int64(len(defaultSources)))
	})

	t.Run("is idempotent for sources", func(t *testing.T) {
		dbPath := t.TempDir() + "/idempotent_test.db"

		db1, err := NewDatabase(dbPath)
		require.NoError(t, err)
		var count1 int64
		db1.DB.Table("sources").Count(&count1)
		db1.Close()

		db2, err := NewDatabase(dbPath)
		require.NoError(t, err)
		defer db2.Close()
		var count2 int64
		db2.DB.Table("sources").Count(&count2)

		assert.Equal(t, count1, count2)
	})
}

func TestClose(t *testing.T) {
	dbPath := t.TempDir() + "/close_test.db"
	db, err := NewDatabase(dbPath)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)
}

func TestPing(t *testing.T) {
	t.Run("returns nil when connected", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		err := db.Ping()
		assert.NoError(t, err)
	})

	t.Run("returns error when closed", func(t *testing.T) {
		dbPath := t.TempDir() + "/ping_test.db"
		db, err := NewDatabase(dbPath)
		require.NoError(t, err)
		db.Close()

		err = db.Ping()
		assert.Error(t, err)
	})
}
