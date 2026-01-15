package favourites

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestDB(t *testing.T) (*gorm.DB, *Repository, func()) {
	dbPath := "./test_favourites_" + t.Name() + ".db"

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

	return db, repo, cleanup
}

func createTestBook(t *testing.T, db *gorm.DB, title string) *entities.Book {
	book := &entities.Book{
		Title:  title,
		Author: "Test Author",
	}
	err := db.Create(book).Error
	require.NoError(t, err)
	return book
}

func createTestHighlight(t *testing.T, db *gorm.DB, bookID uint, text string, isFavorite bool) *entities.Highlight {
	highlight := &entities.Highlight{
		BookID:        bookID,
		Text:          text,
		IsFavorite:    isFavorite,
		HighlightedAt: time.Now(),
	}
	err := db.Create(highlight).Error
	require.NoError(t, err)
	return highlight
}

func TestRepository_SetHighlightFavourite(t *testing.T) {
	db, repo, cleanup := setupTestDB(t)
	defer cleanup()

	book := createTestBook(t, db, "Test Book")
	highlight := createTestHighlight(t, db, book.ID, "Test highlight", false)

	// Set as favourite
	err := repo.SetHighlightFavourite(highlight.ID, true)
	require.NoError(t, err)

	// Verify
	var updated entities.Highlight
	db.First(&updated, highlight.ID)
	assert.True(t, updated.IsFavorite)

	// Unset favourite
	err = repo.SetHighlightFavourite(highlight.ID, false)
	require.NoError(t, err)

	db.First(&updated, highlight.ID)
	assert.False(t, updated.IsFavorite)
}

func TestRepository_GetFavouriteHighlights(t *testing.T) {
	db, repo, cleanup := setupTestDB(t)
	defer cleanup()

	book := createTestBook(t, db, "Test Book")
	createTestHighlight(t, db, book.ID, "Not favourite", false)
	createTestHighlight(t, db, book.ID, "Favourite 1", true)
	createTestHighlight(t, db, book.ID, "Favourite 2", true)

	highlights, total, err := repo.GetFavouriteHighlights(0, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, highlights, 2)
}

func TestRepository_GetFavouriteHighlights_Pagination(t *testing.T) {
	db, repo, cleanup := setupTestDB(t)
	defer cleanup()

	book := createTestBook(t, db, "Test Book")
	for i := 0; i < 5; i++ {
		createTestHighlight(t, db, book.ID, "Favourite", true)
	}

	// Get first page
	highlights, total, err := repo.GetFavouriteHighlights(0, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, highlights, 2)

	// Get second page
	highlights, _, err = repo.GetFavouriteHighlights(0, 2, 2)
	require.NoError(t, err)
	assert.Len(t, highlights, 2)
}

func TestRepository_GetFavouriteHighlightsByBook(t *testing.T) {
	db, repo, cleanup := setupTestDB(t)
	defer cleanup()

	book1 := createTestBook(t, db, "Book 1")
	book2 := createTestBook(t, db, "Book 2")

	createTestHighlight(t, db, book1.ID, "Book1 Fav", true)
	createTestHighlight(t, db, book2.ID, "Book2 Fav", true)
	createTestHighlight(t, db, book1.ID, "Book1 Not Fav", false)

	highlights, err := repo.GetFavouriteHighlightsByBook(book1.ID)

	require.NoError(t, err)
	assert.Len(t, highlights, 1)
	assert.Equal(t, "Book1 Fav", highlights[0].Text)
}

func TestRepository_GetFavouriteCount(t *testing.T) {
	db, repo, cleanup := setupTestDB(t)
	defer cleanup()

	book := createTestBook(t, db, "Test Book")
	createTestHighlight(t, db, book.ID, "Fav 1", true)
	createTestHighlight(t, db, book.ID, "Fav 2", true)
	createTestHighlight(t, db, book.ID, "Not Fav", false)

	count, err := repo.GetFavouriteCount(0)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestRepository_GetHighlightByID(t *testing.T) {
	db, repo, cleanup := setupTestDB(t)
	defer cleanup()

	book := createTestBook(t, db, "Test Book")
	highlight := createTestHighlight(t, db, book.ID, "Test highlight", false)

	result, err := repo.GetHighlightByID(highlight.ID)

	require.NoError(t, err)
	assert.Equal(t, highlight.ID, result.ID)
	assert.Equal(t, "Test highlight", result.Text)
}

func TestRepository_GetHighlightByID_NotFound(t *testing.T) {
	_, repo, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := repo.GetHighlightByID(999)

	assert.Error(t, err)
}
