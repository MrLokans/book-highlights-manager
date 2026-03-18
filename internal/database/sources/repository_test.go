package sources_test

import (
	"os"
	"testing"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/database/sources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestRepo(t *testing.T) (*sources.Repository, func()) {
	t.Helper()
	dbPath := "./test_sources_" + t.Name() + ".db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	repo := sources.NewRepository(db.DB)
	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return repo, cleanup
}

func TestRepository_GetByName(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	t.Run("returns seeded source", func(t *testing.T) {
		source, err := repo.GetByName("kindle")
		require.NoError(t, err)
		assert.Equal(t, "kindle", source.Name)
		assert.Equal(t, "Amazon Kindle", source.DisplayName)
	})

	t.Run("returns error for unknown source", func(t *testing.T) {
		_, err := repo.GetByName("unknown_source")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func TestRepository_GetAll(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	srcs, err := repo.GetAll()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(srcs), 5)

	names := make(map[string]bool)
	for _, s := range srcs {
		names[s.Name] = true
	}
	for _, expected := range []string{"readwise", "kindle", "apple_books", "moonreader"} {
		assert.True(t, names[expected], "expected source %s not found", expected)
	}
}
