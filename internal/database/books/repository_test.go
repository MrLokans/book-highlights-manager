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

	t.Run("skips permanently deleted highlights and resolves highlight sources on re-import", func(t *testing.T) {
		repo := setupRepo(t)
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		highlights := func() []entities.Highlight {
			return []entities.Highlight{
				{Text: "keep", HighlightedAt: at, Source: entities.Source{Name: "kindle"}},
				{Text: "gone", HighlightedAt: at, Source: entities.Source{Name: "kindle"}},
			}
		}

		book := &entities.Book{Title: "Del HL", Author: "Author", Highlights: highlights()}
		require.NoError(t, repo.SaveBook(book))
		require.NoError(t, repo.DeleteHighlightPermanently(book.Highlights[1].ID))

		require.NoError(t, repo.SaveBook(&entities.Book{Title: "Del HL", Author: "Author", Highlights: highlights()}))

		retrieved, err := repo.GetBookByID(book.ID)
		require.NoError(t, err)
		require.Len(t, retrieved.Highlights, 1)
		assert.Equal(t, "keep", retrieved.Highlights[0].Text)
		assert.NotZero(t, retrieved.Highlights[0].SourceID)
	})

	t.Run("imports and re-imports a book with more highlights than SQLite's parameter limit", func(t *testing.T) {
		repo := setupRepo(t)
		highlights := func() []entities.Highlight {
			hs := make([]entities.Highlight, 3000)
			for i := range hs {
				hs[i] = entities.Highlight{Text: fmt.Sprint(i), HighlightedAt: time.Unix(0, 0)}
			}
			return hs
		}

		require.NoError(t, repo.SaveBook(&entities.Book{Title: "Big", Author: "Author", Highlights: highlights()}))
		require.NoError(t, repo.SaveBook(&entities.Book{Title: "Big", Author: "Author", Highlights: highlights()}))

		_, total, err := repo.GetStats()
		require.NoError(t, err)
		assert.Equal(t, int64(3000), total)
	})

	t.Run("skips permanently deleted book", func(t *testing.T) {
		repo := setupRepo(t)

		book := &entities.Book{Title: "Delete Me", Author: "Author"}
		require.NoError(t, repo.SaveBook(book))
		require.NoError(t, repo.DeleteBookPermanently(book.ID))

		// Try to save again — should be silently skipped
		book2 := &entities.Book{Title: "Delete Me", Author: "Author"}
		require.NoError(t, repo.SaveBook(book2))
		assert.Zero(t, book2.ID)
	})

	t.Run("re-import keeps enriched metadata and created_at", func(t *testing.T) {
		repo := setupRepo(t)
		highlightedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

		book := &entities.Book{Title: "Enriched", Author: "Author", Highlights: []entities.Highlight{{Text: "h1", HighlightedAt: highlightedAt}}}
		require.NoError(t, repo.SaveBook(book))
		require.NoError(t, repo.UpdateBookMetadata(book.ID, map[string]any{
			"cover_url": "http://cover", "publisher": "Pub", "isbn": "123", "publication_year": 2001,
		}))
		original, err := repo.GetBookByID(book.ID)
		require.NoError(t, err)

		reimport := &entities.Book{Title: "Enriched", Author: "Author", Highlights: []entities.Highlight{
			{Text: "h1", HighlightedAt: highlightedAt},
			{Text: "h2", HighlightedAt: highlightedAt},
		}}
		require.NoError(t, repo.SaveBook(reimport))

		got, err := repo.GetBookByID(book.ID)
		require.NoError(t, err)
		assert.Equal(t, "http://cover", got.CoverURL)
		assert.Equal(t, "Pub", got.Publisher)
		assert.Equal(t, "123", got.ISBN)
		assert.Equal(t, 2001, got.PublicationYear)
		assert.True(t, original.CreatedAt.Equal(got.CreatedAt), "created_at changed: %v -> %v", original.CreatedAt, got.CreatedAt)
		assert.Len(t, got.Highlights, 2)
	})

	t.Run("re-import overwrites metadata the source does provide", func(t *testing.T) {
		repo := setupRepo(t)

		book := &entities.Book{Title: "Cover", Author: "Author", CoverURL: "http://old"}
		require.NoError(t, repo.SaveBook(book))
		require.NoError(t, repo.SaveBook(&entities.Book{Title: "Cover", Author: "Author", CoverURL: "http://new"}))

		got, err := repo.GetBookByID(book.ID)
		require.NoError(t, err)
		assert.Equal(t, "http://new", got.CoverURL)
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
