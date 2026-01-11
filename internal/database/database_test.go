package database

import (
	"os"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {
	// Create a temporary database file
	dbPath := "./test.db"
	defer os.Remove(dbPath)

	// Initialize database
	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer db.Close()

	t.Run("SaveBook creates new book", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Test Book",
			Author: "Test Author",
			File:   "test.epub",
			Highlights: []entities.Highlight{
				{
					Time: time.Now().Format(time.RFC3339),
					Text: "This is a test highlight",
					Page: 42,
				},
			},
		}

		err := db.SaveBook(book)
		assert.NoError(t, err)
		assert.NotZero(t, book.ID)
		assert.NotZero(t, book.Highlights[0].ID)
		assert.Equal(t, book.ID, book.Highlights[0].BookID)
	})

	t.Run("GetBookByTitleAndAuthor retrieves saved book", func(t *testing.T) {
		retrievedBook, err := db.GetBookByTitleAndAuthor("Test Book", "Test Author")
		assert.NoError(t, err)
		assert.Equal(t, "Test Book", retrievedBook.Title)
		assert.Equal(t, "Test Author", retrievedBook.Author)
		assert.Len(t, retrievedBook.Highlights, 1)
		assert.Equal(t, "This is a test highlight", retrievedBook.Highlights[0].Text)
		assert.Equal(t, 42, retrievedBook.Highlights[0].Page) //nolint:staticcheck // Testing deprecated field for backward compatibility
	})

	t.Run("GetAllBooks returns all saved books", func(t *testing.T) {
		// Save another book
		book2 := &entities.Book{
			Title:  "Another Book",
			Author: "Another Author",
			File:   "another.epub",
			Highlights: []entities.Highlight{
				{
					Time: time.Now().Format(time.RFC3339),
					Text: "Another highlight",
					Page: 100,
				},
			},
		}

		err := db.SaveBook(book2)
		require.NoError(t, err)

		books, err := db.GetAllBooks()
		assert.NoError(t, err)
		assert.Len(t, books, 2)
	})

	t.Run("SaveBook updates existing book", func(t *testing.T) {
		// Get the existing book
		existingBook, err := db.GetBookByTitleAndAuthor("Test Book", "Test Author")
		require.NoError(t, err)

		// Add another highlight
		newHighlight := entities.Highlight{
			Time: time.Now().Format(time.RFC3339),
			Text: "Second highlight",
			Page: 84,
		}
		existingBook.Highlights = append(existingBook.Highlights, newHighlight)

		// Save the updated book
		err = db.SaveBook(existingBook)
		assert.NoError(t, err)

		// Retrieve and verify
		updatedBook, err := db.GetBookByTitleAndAuthor("Test Book", "Test Author")
		assert.NoError(t, err)
		assert.Len(t, updatedBook.Highlights, 2)
	})
}
