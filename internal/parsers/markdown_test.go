package parsers

import (
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownParser(t *testing.T) {
	parser := NewMarkdownParser("../fixtures/exported")

	t.Run("ParseAllMarkdownFiles", func(t *testing.T) {
		books, result, err := parser.ParseAllMarkdownFiles()
		require.NoError(t, err)

		assert.Greater(t, result.BooksProcessed, 0, "Should process at least one book")
		assert.Equal(t, len(books), result.BooksProcessed, "Books count should match processed count")
		assert.Greater(t, result.HighlightsProcessed, 0, "Should process at least one highlight")

		t.Logf("Parsed %d books with %d highlights total", result.BooksProcessed, result.HighlightsProcessed)

		// Verify each book has required fields
		for _, book := range books {
			assert.NotEmpty(t, book.Title, "Book should have a title")
			assert.NotEmpty(t, book.Author, "Book should have an author")
			assert.Greater(t, len(book.Highlights), 0, "Book should have highlights")

			// Verify highlights have content
			for _, highlight := range book.Highlights {
				assert.NotEmpty(t, highlight.Text, "Highlight should have text")
				assert.NotEmpty(t, highlight.Time, "Highlight should have time")
			}
		}
	})

	t.Run("ParseSpecificFile", func(t *testing.T) {
		// Test parsing the Database Internals file specifically
		book, err := parser.ParseMarkdownFile("../fixtures/exported/Database Internals.md")
		require.NoError(t, err)

		assert.Equal(t, "Database Internals", book.Title)
		assert.Equal(t, "Alex  Petrov", book.Author)
		assert.Greater(t, len(book.Highlights), 0, "Should have highlights")

		// Check first highlight
		firstHighlight := book.Highlights[0]
		assert.Equal(t, "2025-02-13T07:34:47+01:00", firstHighlight.Time)
		assert.Contains(t, firstHighlight.Text, "Since row-oriented stores")

		t.Logf("Parsed book '%s' by %s with %d highlights", book.Title, book.Author, len(book.Highlights))
	})

	t.Run("ParseBuildTonyFadellFile", func(t *testing.T) {
		// Test parsing the Build - Tony Fadell file which has a different format
		book, err := parser.ParseMarkdownFile("../fixtures/exported/Build - Tony Fadell.md")
		require.NoError(t, err)

		assert.Equal(t, "Build", book.Title)
		assert.Equal(t, "Tony Fadell", book.Author)
		assert.Greater(t, len(book.Highlights), 0, "Should have highlights")

		// Check first highlight (different timestamp format)
		firstHighlight := book.Highlights[0]
		assert.Equal(t, "2022-10-02 08:07:58.549075", firstHighlight.Time)
		assert.Contains(t, firstHighlight.Text, "What do I want to learn")

		t.Logf("Parsed book '%s' by %s with %d highlights", book.Title, book.Author, len(book.Highlights))
	})

	t.Run("CompareWithDatabase", func(t *testing.T) {
		// Parse markdown books
		markdownBooks, _, err := parser.ParseAllMarkdownFiles()
		require.NoError(t, err)

		// Create some mock database books for comparison
		dbBooks := []entities.Book{
			{
				Title:  "Database Internals",
				Author: "Alex  Petrov",
				Highlights: []entities.Highlight{
					{Text: "Test highlight 1", Time: "2025-01-01T00:00:00Z"},
					{Text: "Test highlight 2", Time: "2025-01-02T00:00:00Z"},
				},
			},
			{
				Title:  "Only in Database",
				Author: "Test Author",
				Highlights: []entities.Highlight{
					{Text: "Database only highlight", Time: "2025-01-01T00:00:00Z"},
				},
			},
		}

		result := parser.CompareWithDatabase(markdownBooks, dbBooks)

		assert.Equal(t, len(markdownBooks), result.MarkdownBooks)
		assert.Equal(t, len(dbBooks), result.DatabaseBooks)
		assert.Greater(t, len(result.Matches), 0, "Should find at least one match")
		assert.Greater(t, len(result.OnlyInMarkdown), 0, "Should find books only in markdown")
		assert.Greater(t, len(result.OnlyInDatabase), 0, "Should find books only in database")

		t.Logf("Comparison result: %d matches, %d only in markdown, %d only in database",
			len(result.Matches), len(result.OnlyInMarkdown), len(result.OnlyInDatabase))

		// Verify the match details
		for _, match := range result.Matches {
			assert.NotEmpty(t, match.Title)
			assert.NotEmpty(t, match.Author)
			assert.Greater(t, match.MarkdownHighlights, 0)
			assert.Greater(t, match.DatabaseHighlights, 0)
		}
	})

	t.Run("BookExists", func(t *testing.T) {
		// Test with a book that should exist
		existingBook := entities.Book{
			Title:  "Database Internals",
			Author: "Alex  Petrov",
		}
		assert.True(t, parser.BookExists(existingBook), "Should find existing book file")

		// Test with a book that doesn't exist
		nonExistentBook := entities.Book{
			Title:  "Non Existent Book",
			Author: "Unknown Author",
		}
		assert.False(t, parser.BookExists(nonExistentBook), "Should not find non-existent book file")
	})
}
