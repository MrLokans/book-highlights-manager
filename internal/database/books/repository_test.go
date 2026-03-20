package books_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/database/books"
	"github.com/mrlokans/assistant/internal/database/sources"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRepo(t *testing.T) *books.Repository {
	t.Helper()
	dbPath := t.TempDir() + "/test_books.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	sourcesRepo := sources.NewRepository(db.DB)
	return books.NewRepository(db.DB, sourcesRepo)
}

func TestRepository_SaveBook(t *testing.T) {
	t.Run("creates a new book", func(t *testing.T) {
		repo := setupRepo(t)

		book := &entities.Book{Title: "Test Book", Author: "Author"}
		err := repo.SaveBook(book)
		require.NoError(t, err)
		assert.NotZero(t, book.ID)
	})

	t.Run("upserts existing book and merges highlights", func(t *testing.T) {
		repo := setupRepo(t)

		book := &entities.Book{
			Title:  "Upsert Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "highlight 1", HighlightedAt: time.Now()},
			},
		}
		require.NoError(t, repo.SaveBook(book))
		origID := book.ID

		// Save again with new highlight
		book2 := &entities.Book{
			Title:  "Upsert Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "highlight 1", HighlightedAt: book.Highlights[0].HighlightedAt},
				{Text: "highlight 2", HighlightedAt: time.Now()},
			},
		}
		require.NoError(t, repo.SaveBook(book2))
		assert.Equal(t, origID, book2.ID)

		retrieved, err := repo.GetBookByID(origID)
		require.NoError(t, err)
		assert.Len(t, retrieved.Highlights, 2)
	})

	t.Run("resolves source by name", func(t *testing.T) {
		repo := setupRepo(t)

		book := &entities.Book{
			Title:  "Source Book",
			Author: "Author",
			Source: entities.Source{Name: "kindle"},
		}
		require.NoError(t, repo.SaveBook(book))
		assert.NotZero(t, book.SourceID)
	})

	t.Run("skips permanently deleted book", func(t *testing.T) {
		repo := setupRepo(t)

		book := &entities.Book{Title: "Delete Me", Author: "Author"}
		require.NoError(t, repo.SaveBook(book))
		require.NoError(t, repo.DeleteBookPermanently(book.ID, 0))

		// Try to save again — should be silently skipped
		book2 := &entities.Book{Title: "Delete Me", Author: "Author"}
		require.NoError(t, repo.SaveBook(book2))
		assert.Zero(t, book2.ID)
	})
}

func TestRepository_GetBookByID(t *testing.T) {
	repo := setupRepo(t)

	book := &entities.Book{Title: "ID Book", Author: "Author"}
	require.NoError(t, repo.SaveBook(book))

	retrieved, err := repo.GetBookByID(book.ID)
	require.NoError(t, err)
	assert.Equal(t, "ID Book", retrieved.Title)

	_, err = repo.GetBookByID(99999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRepository_GetBookByID_ReturnsAllHighlights(t *testing.T) {
	repo := setupRepo(t)

	book := &entities.Book{
		Title:  "Many Highlights",
		Author: "Author",
	}
	for i := 0; i < 22; i++ {
		book.Highlights = append(book.Highlights, entities.Highlight{
			Text:          fmt.Sprintf("Highlight %d", i+1),
			LocationType:  entities.LocationTypePage,
			LocationValue: i + 1,
			HighlightedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}
	require.NoError(t, repo.SaveBook(book))

	retrieved, err := repo.GetBookByID(book.ID)
	require.NoError(t, err)
	assert.Len(t, retrieved.Highlights, 22, "GetBookByID must return all highlights, not just 1")
}

func TestRepository_GetAllBooks(t *testing.T) {
	repo := setupRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Book A", Author: "Author"}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Book B", Author: "Author"}))

	all, err := repo.GetAllBooks()
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestRepository_SearchBooks(t *testing.T) {
	repo := setupRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Go Programming", Author: "Author"}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Python Cookbook", Author: "Author"}))

	results, err := repo.SearchBooks("Go")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Go Programming", results[0].Title)
}

func TestRepository_DeleteBook(t *testing.T) {
	repo := setupRepo(t)

	book := &entities.Book{Title: "Soft Delete", Author: "Author"}
	require.NoError(t, repo.SaveBook(book))

	err := repo.DeleteBook(book.ID)
	require.NoError(t, err)

	_, err = repo.GetBookByID(book.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRepository_GetStats(t *testing.T) {
	repo := setupRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{
		Title:      "Stats Book",
		Author:     "Author",
		Highlights: []entities.Highlight{{Text: "h1"}, {Text: "h2"}},
	}))

	totalBooks, totalHighlights, err := repo.GetStats()
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalBooks)
	assert.Equal(t, int64(2), totalHighlights)
}

func TestRepository_UpdateBookMetadata(t *testing.T) {
	repo := setupRepo(t)

	book := &entities.Book{Title: "Meta Book", Author: "Author"}
	require.NoError(t, repo.SaveBook(book))

	err := repo.UpdateBookMetadata(book.ID, map[string]any{
		"cover_url": "https://example.com/cover.jpg",
		"publisher": "Test Publisher",
	})
	require.NoError(t, err)

	updated, err := repo.GetBookByID(book.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/cover.jpg", updated.CoverURL)
	assert.Equal(t, "Test Publisher", updated.Publisher)
}

func TestRepository_GetBookByTitleAndAuthor(t *testing.T) {
	repo := setupRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "FindMe", Author: "Author X"}))

	book, err := repo.GetBookByTitleAndAuthor("FindMe", "Author X")
	require.NoError(t, err)
	assert.Equal(t, "FindMe", book.Title)

	_, err = repo.GetBookByTitleAndAuthor("Nonexistent", "Nobody")
	assert.Error(t, err)
}

func TestRepository_GetAllBooksForUser(t *testing.T) {
	repo := setupRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "User Book", Author: "A", UserID: 1}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Other Book", Author: "A", UserID: 2}))

	books, err := repo.GetAllBooksForUser(1)
	require.NoError(t, err)
	assert.Len(t, books, 1)
	assert.Equal(t, "User Book", books[0].Title)
}

func TestRepository_SaveBookForUser(t *testing.T) {
	repo := setupRepo(t)

	book := &entities.Book{Title: "ForUser", Author: "A"}
	require.NoError(t, repo.SaveBookForUser(book, 42))
	assert.Equal(t, uint(42), book.UserID)
}

func TestRepository_GetBooksMissingMetadata(t *testing.T) {
	repo := setupRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "No Meta", Author: "A"}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Has Meta", Author: "A", CoverURL: "url", Publisher: "Pub", PublicationYear: 2020}))

	missing, err := repo.GetBooksMissingMetadata()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(missing), 1)

	found := false
	for _, b := range missing {
		if b.Title == "No Meta" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestRepository_FindBookByISBN(t *testing.T) {
	repo := setupRepo(t)

	book := &entities.Book{Title: "ISBN Book", Author: "A", ISBN: "978-0-13-468599-1", UserID: 1}
	require.NoError(t, repo.SaveBook(book))

	found, err := repo.FindBookByISBN("978-0-13-468599-1", 1)
	require.NoError(t, err)
	assert.Equal(t, "ISBN Book", found.Title)

	_, err = repo.FindBookByISBN("nonexistent", 1)
	assert.Error(t, err)
}

func TestRepository_GetStatsForUser(t *testing.T) {
	repo := setupRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{
		Title: "User Stats", Author: "A", UserID: 5,
		Highlights: []entities.Highlight{
			{Text: "h1", UserID: 5},
			{Text: "h2", UserID: 5},
		},
	}))

	totalBooks, totalHighlights, err := repo.GetStatsForUser(5)
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalBooks)
	assert.Equal(t, int64(2), totalHighlights)
}

func TestRepository_Highlights(t *testing.T) {
	repo := setupRepo(t)

	book := &entities.Book{
		Title:      "Highlight Book",
		Author:     "Author",
		Highlights: []entities.Highlight{{Text: "test highlight"}},
	}
	require.NoError(t, repo.SaveBook(book))

	t.Run("GetHighlightByID", func(t *testing.T) {
		h, err := repo.GetHighlightByID(book.Highlights[0].ID)
		require.NoError(t, err)
		assert.Equal(t, "test highlight", h.Text)
	})

	t.Run("GetHighlightsForBook", func(t *testing.T) {
		highlights, err := repo.GetHighlightsForBook(book.ID)
		require.NoError(t, err)
		assert.Len(t, highlights, 1)
	})

	t.Run("DeleteHighlight soft deletes", func(t *testing.T) {
		err := repo.DeleteHighlight(book.Highlights[0].ID)
		require.NoError(t, err)

		_, err = repo.GetHighlightByID(book.Highlights[0].ID)
		assert.Error(t, err)
	})
}
