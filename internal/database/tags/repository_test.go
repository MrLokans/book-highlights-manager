package tags

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestDB(t *testing.T) (*Repository, func()) {
	dbPath := "./test_tags_" + t.Name() + ".db"

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&entities.Source{},
		&entities.User{},
		&entities.Book{},
		&entities.Highlight{},
		&entities.Tag{},
	)
	require.NoError(t, err)

	repo := NewRepository(db)

	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		os.Remove(dbPath)
	}

	return repo, cleanup
}

func TestRepository_CreateTag(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	tag, err := repo.CreateTag("fiction", 1)

	require.NoError(t, err)
	assert.NotZero(t, tag.ID)
	assert.Equal(t, "fiction", tag.Name)
	assert.Equal(t, uint(1), tag.UserID)
}

func TestRepository_GetOrCreateTag_New(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	tag, err := repo.GetOrCreateTag("science", 1)

	require.NoError(t, err)
	assert.NotZero(t, tag.ID)
	assert.Equal(t, "science", tag.Name)
}

func TestRepository_GetOrCreateTag_Existing(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Create first
	tag1, err := repo.CreateTag("history", 1)
	require.NoError(t, err)

	// Get or create should return existing
	tag2, err := repo.GetOrCreateTag("history", 1)
	require.NoError(t, err)
	assert.Equal(t, tag1.ID, tag2.ID)
}

func TestRepository_GetOrCreateTag_CaseInsensitive(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	tag1, err := repo.CreateTag("Fiction", 1)
	require.NoError(t, err)

	// Should find existing despite different case
	tag2, err := repo.GetOrCreateTag("fiction", 1)
	require.NoError(t, err)
	assert.Equal(t, tag1.ID, tag2.ID)
}

func TestRepository_GetTagsForUser(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Create tags for different users
	_, err := repo.CreateTag("user1-tag", 1)
	require.NoError(t, err)
	_, err = repo.CreateTag("user2-tag", 2)
	require.NoError(t, err)

	// Get tags for user 1
	tags, err := repo.GetTagsForUser(1)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.Equal(t, "user1-tag", tags[0].Name)
}

func TestRepository_SearchTags(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := repo.CreateTag("science-fiction", 1)
	require.NoError(t, err)
	_, err = repo.CreateTag("history", 1)
	require.NoError(t, err)

	// Search for "fic"
	tags, err := repo.SearchTags("fic", 1)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.Equal(t, "science-fiction", tags[0].Name)
}

func TestRepository_DeleteTag(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	tag, err := repo.CreateTag("to-delete", 1)
	require.NoError(t, err)

	err = repo.DeleteTag(tag.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = repo.GetTagByID(tag.ID)
	assert.Error(t, err)
}

func TestRepository_IsTagOrphan(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	tag, err := repo.CreateTag("orphan-tag", 1)
	require.NoError(t, err)

	// Tag with no associations should be orphan
	isOrphan, err := repo.IsTagOrphan(tag.ID)
	require.NoError(t, err)
	assert.True(t, isOrphan)
}

func TestRepository_DeleteOrphanTags(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Create orphan tags
	_, err := repo.CreateTag("orphan1", 1)
	require.NoError(t, err)
	_, err = repo.CreateTag("orphan2", 1)
	require.NoError(t, err)

	deleted, err := repo.DeleteOrphanTags()
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	// Verify tags are gone
	tags, err := repo.GetTagsForUser(1)
	require.NoError(t, err)
	assert.Empty(t, tags)
}
