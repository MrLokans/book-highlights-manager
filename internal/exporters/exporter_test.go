package exporters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- GenerateMarkdown Tests ---

func TestGenerateMarkdown(t *testing.T) {
	t.Run("generates basic markdown with frontmatter", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Test Book",
			Author: "Test Author",
			Source: entities.Source{Name: "kindle"},
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "---")
		assert.Contains(t, markdown, "title: \"Test Book\"")
		assert.Contains(t, markdown, "author: \"Test Author\"")
		assert.Contains(t, markdown, "content_source: kindle")
		assert.Contains(t, markdown, "content_type: book_highlights")
		assert.Contains(t, markdown, "tags: [")
		assert.Contains(t, markdown, "## Highlights")
	})

	t.Run("uses unknown source when not specified", func(t *testing.T) {
		book := &entities.Book{
			Title:  "No Source Book",
			Author: "Author",
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "content_source: unknown")
	})

	t.Run("includes highlights with timestamps", func(t *testing.T) {
		highlightTime := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
		book := &entities.Book{
			Title:  "Highlight Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{
					Text:          "This is a highlight",
					HighlightedAt: highlightTime,
				},
			},
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "> [!quote] 2024-06-15 14:30")
		assert.Contains(t, markdown, "> This is a highlight")
	})

	t.Run("uses deprecated Time field as fallback", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Legacy Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{
					Text: "Legacy highlight",
					Time: "2023-01-01",
				},
			},
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "> [!quote] 2023-01-01")
	})

	t.Run("includes notes when present", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Notes Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{
					Text: "Highlighted text",
					Note: "My personal note",
				},
			},
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "> Highlighted text")
		assert.Contains(t, markdown, "**Note:** My personal note")
	})

	t.Run("escapes quotes in title and author", func(t *testing.T) {
		book := &entities.Book{
			Title:  `Book with "Quotes"`,
			Author: `Author "Name"`,
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, `title: "Book with \"Quotes\""`)
		assert.Contains(t, markdown, `author: "Author \"Name\""`)
	})

	t.Run("handles multiline highlights", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Multiline Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{
					Text: "Line 1\nLine 2\nLine 3",
				},
			},
		}

		markdown := GenerateMarkdown(book)

		// Each line should be prefixed with >
		assert.Contains(t, markdown, "> Line 1\n> Line 2\n> Line 3")
	})

	t.Run("handles multiple highlights", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Multi Highlight Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "First highlight"},
				{Text: "Second highlight"},
				{Text: "Third highlight"},
			},
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "> First highlight")
		assert.Contains(t, markdown, "> Second highlight")
		assert.Contains(t, markdown, "> Third highlight")
	})

	t.Run("includes created_at date", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Date Book",
			Author: "Author",
		}

		markdown := GenerateMarkdown(book)

		today := time.Now().Format("2006-01-02")
		assert.Contains(t, markdown, "created_at: "+today)
	})
}

// --- MarkdownExporter Tests ---

func TestMarkdownExporter(t *testing.T) {
	t.Run("NewMarkdownExporter initializes correctly", func(t *testing.T) {
		exporter := NewMarkdownExporter("/vault")

		assert.Equal(t, "/vault", exporter.ExportDir)
		assert.Equal(t, "index.md", exporter.IndexFileName)
	})

	t.Run("Export fails when export directory does not exist", func(t *testing.T) {
		exporter := NewMarkdownExporter("/nonexistent/path")

		books := []entities.Book{{Title: "Test", Author: "Author"}}
		_, err := exporter.Export(books)

		assert.Error(t, err)
	})

	t.Run("Export creates export directory and files", func(t *testing.T) {
		// Create temp directory
		tempDir := t.TempDir()

		exporter := NewMarkdownExporter(tempDir)

		books := []entities.Book{
			{
				Title:  "Export Test Book",
				Author: "Export Author",
				Source: entities.Source{Name: "kindle"},
				Highlights: []entities.Highlight{
					{Text: "A highlight", Time: "2024-01-01"},
				},
			},
		}

		result, err := exporter.Export(books)
		require.NoError(t, err)

		assert.Equal(t, 1, result.BooksProcessed)
		assert.Equal(t, 1, result.HighlightsProcessed)
		assert.Equal(t, 0, result.BooksFailed)

		// Verify file was created
		expectedPath := filepath.Join(tempDir, "kindle", "Export Test Book.md")
		_, err = os.Stat(expectedPath)
		assert.NoError(t, err)

		// Verify file content
		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "title: \"Export Test Book\"")
		assert.Contains(t, string(content), "author: \"Export Author\"")
	})

	t.Run("Export uses unknown folder for books without source", func(t *testing.T) {
		tempDir := t.TempDir()
		exporter := NewMarkdownExporter(tempDir)

		books := []entities.Book{
			{
				Title:  "No Source Book",
				Author: "Author",
			},
		}

		_, err := exporter.Export(books)
		require.NoError(t, err)

		expectedPath := filepath.Join(tempDir, "unknown", "No Source Book.md")
		_, err = os.Stat(expectedPath)
		assert.NoError(t, err)
	})

	t.Run("Export handles multiple books", func(t *testing.T) {
		tempDir := t.TempDir()
		exporter := NewMarkdownExporter(tempDir)

		books := []entities.Book{
			{Title: "Book 1", Author: "Author 1", Source: entities.Source{Name: "kindle"}},
			{Title: "Book 2", Author: "Author 2", Source: entities.Source{Name: "apple_books"}},
			{Title: "Book 3", Author: "Author 3", Source: entities.Source{Name: "kindle"}},
		}

		result, err := exporter.Export(books)
		require.NoError(t, err)

		assert.Equal(t, 3, result.BooksProcessed)

		// Verify files created
		assert.FileExists(t, filepath.Join(tempDir, "kindle", "Book 1.md"))
		assert.FileExists(t, filepath.Join(tempDir, "apple_books", "Book 2.md"))
		assert.FileExists(t, filepath.Join(tempDir, "kindle", "Book 3.md"))
	})

	t.Run("Export counts highlights correctly", func(t *testing.T) {
		tempDir := t.TempDir()
		exporter := NewMarkdownExporter(tempDir)

		books := []entities.Book{
			{
				Title:  "Many Highlights",
				Author: "Author",
				Highlights: []entities.Highlight{
					{Text: "Highlight 1"},
					{Text: "Highlight 2"},
					{Text: "Highlight 3"},
				},
			},
		}

		result, err := exporter.Export(books)
		require.NoError(t, err)

		assert.Equal(t, 3, result.HighlightsProcessed)
	})

	t.Run("Export resets result state between calls", func(t *testing.T) {
		tempDir := t.TempDir()
		exporter := NewMarkdownExporter(tempDir)

		books1 := []entities.Book{{Title: "Book 1", Author: "Author"}}
		result1, err := exporter.Export(books1)
		require.NoError(t, err)
		assert.Equal(t, 1, result1.BooksProcessed)

		books2 := []entities.Book{{Title: "Book 2", Author: "Author"}}
		result2, err := exporter.Export(books2)
		require.NoError(t, err)
		assert.Equal(t, 1, result2.BooksProcessed) // Should be 1, not 2
	})
}

// --- DatabaseMarkdownExporter Tests ---

func setupTestDatabase(t *testing.T) (*database.Database, func()) {
	t.Helper()
	dbPath := "./test_exporter_" + strings.ReplaceAll(t.Name(), "/", "_") + ".db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, cleanup
}

func TestDatabaseMarkdownExporter(t *testing.T) {
	t.Run("NewDatabaseMarkdownExporter initializes correctly", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		tempDir := t.TempDir()
		exporter := NewDatabaseMarkdownExporter(db, tempDir)

		assert.NotNil(t, exporter)
		assert.NotNil(t, exporter.db)
		assert.NotNil(t, exporter.markdownExporter)
	})

	t.Run("Export saves to database and exports markdown", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		tempDir := t.TempDir()
		exporter := NewDatabaseMarkdownExporter(db, tempDir)

		books := []entities.Book{
			{
				Title:  "DB Export Book",
				Author: "DB Author",
				Source: entities.Source{Name: "kindle"},
				Highlights: []entities.Highlight{
					{Text: "Database highlight"},
				},
			},
		}

		result, err := exporter.Export(books)
		require.NoError(t, err)

		assert.Equal(t, 1, result.BooksProcessed)
		assert.Equal(t, 1, result.HighlightsProcessed)

		// Verify book was saved to database
		savedBook, err := db.GetBookByTitleAndAuthor("DB Export Book", "DB Author")
		require.NoError(t, err)
		assert.Equal(t, "DB Export Book", savedBook.Title)

		// Verify markdown file was created
		expectedPath := filepath.Join(tempDir, "kindle", "DB Export Book.md")
		assert.FileExists(t, expectedPath)
	})

	t.Run("Export continues after database save failure", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		tempDir := t.TempDir()
		exporter := NewDatabaseMarkdownExporter(db, tempDir)

		// Save first book to database
		firstBook := &entities.Book{
			Title:  "First Book",
			Author: "Author",
		}
		err := db.SaveBook(firstBook)
		require.NoError(t, err)

		// Try to export books - one already exists, should update
		books := []entities.Book{
			{Title: "First Book", Author: "Author", Highlights: []entities.Highlight{{Text: "New highlight"}}},
			{Title: "Second Book", Author: "Author"},
		}

		result, err := exporter.Export(books)
		require.NoError(t, err)

		// Both should be processed (first updates, second creates)
		assert.Equal(t, 2, result.BooksProcessed)
	})

	t.Run("GetAllBooks retrieves books", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		tempDir := t.TempDir()
		exporter := NewDatabaseMarkdownExporter(db, tempDir)

		// Save some books directly
		book1 := &entities.Book{Title: "Book 1", Author: "Author"}
		book2 := &entities.Book{Title: "Book 2", Author: "Author"}
		require.NoError(t, db.SaveBook(book1))
		require.NoError(t, db.SaveBook(book2))

		books, err := exporter.GetAllBooks()
		require.NoError(t, err)
		assert.Len(t, books, 2)
	})

	t.Run("GetBookByTitleAndAuthor retrieves specific book", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		tempDir := t.TempDir()
		exporter := NewDatabaseMarkdownExporter(db, tempDir)

		// Save a book
		book := &entities.Book{Title: "Specific Book", Author: "Specific Author"}
		require.NoError(t, db.SaveBook(book))

		retrieved, err := exporter.GetBookByTitleAndAuthor("Specific Book", "Specific Author")
		require.NoError(t, err)
		assert.Equal(t, "Specific Book", retrieved.Title)
	})

	t.Run("GetBookByID retrieves by ID", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		tempDir := t.TempDir()
		exporter := NewDatabaseMarkdownExporter(db, tempDir)

		// Save a book
		book := &entities.Book{Title: "ID Book", Author: "ID Author"}
		require.NoError(t, db.SaveBook(book))

		retrieved, err := exporter.GetBookByID(book.ID)
		require.NoError(t, err)
		assert.Equal(t, "ID Book", retrieved.Title)
	})

	t.Run("SearchBooks finds books by query", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		tempDir := t.TempDir()
		exporter := NewDatabaseMarkdownExporter(db, tempDir)

		// Save books
		require.NoError(t, db.SaveBook(&entities.Book{Title: "Programming Go", Author: "Author"}))
		require.NoError(t, db.SaveBook(&entities.Book{Title: "Python Cookbook", Author: "Author"}))

		results, err := exporter.SearchBooks("Go")
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Programming Go", results[0].Title)
	})

	t.Run("Export fails when vault directory does not exist", func(t *testing.T) {
		db, cleanup := setupTestDatabase(t)
		defer cleanup()

		exporter := NewDatabaseMarkdownExporter(db, "/nonexistent/path")

		books := []entities.Book{{Title: "Test", Author: "Author"}}
		_, err := exporter.Export(books)

		assert.Error(t, err)
	})
}

// --- ExportResult Tests ---

func TestExportResult(t *testing.T) {
	t.Run("ExportResult fields are correctly initialized", func(t *testing.T) {
		result := ExportResult{
			BooksProcessed:      10,
			HighlightsProcessed: 100,
			BooksFailed:         2,
			HighlightsFailed:    5,
		}

		assert.Equal(t, 10, result.BooksProcessed)
		assert.Equal(t, 100, result.HighlightsProcessed)
		assert.Equal(t, 2, result.BooksFailed)
		assert.Equal(t, 5, result.HighlightsFailed)
	})

	t.Run("zero value ExportResult has all zeros", func(t *testing.T) {
		result := ExportResult{}

		assert.Equal(t, 0, result.BooksProcessed)
		assert.Equal(t, 0, result.HighlightsProcessed)
		assert.Equal(t, 0, result.BooksFailed)
		assert.Equal(t, 0, result.HighlightsFailed)
	})
}

// --- Edge Cases ---

func TestExporterEdgeCases(t *testing.T) {
	t.Run("GenerateMarkdown handles empty book", func(t *testing.T) {
		book := &entities.Book{}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "---")
		assert.Contains(t, markdown, "## Highlights")
	})

	t.Run("GenerateMarkdown handles book with no highlights", func(t *testing.T) {
		book := &entities.Book{
			Title:      "No Highlights",
			Author:     "Author",
			Highlights: []entities.Highlight{},
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "title: \"No Highlights\"")
		assert.Contains(t, markdown, "## Highlights")
		// Should end without any highlight entries
		highlightSection := strings.Split(markdown, "## Highlights")[1]
		assert.Equal(t, "\n\n", highlightSection)
	})

	t.Run("Export handles empty book list", func(t *testing.T) {
		tempDir := t.TempDir()
		exporter := NewMarkdownExporter(tempDir)

		result, err := exporter.Export([]entities.Book{})
		require.NoError(t, err)

		assert.Equal(t, 0, result.BooksProcessed)
		assert.Equal(t, 0, result.HighlightsProcessed)
	})

	t.Run("GenerateMarkdown handles special characters in text", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Special <Characters> & \"Quotes\"",
			Author: "Author's Name",
			Highlights: []entities.Highlight{
				{Text: "Text with *markdown* and `code`"},
			},
		}

		markdown := GenerateMarkdown(book)

		// Title should be quoted and escaped
		assert.Contains(t, markdown, `title: "Special <Characters> & \"Quotes\""`)
		// Highlight text should be preserved
		assert.Contains(t, markdown, "Text with *markdown* and `code`")
	})

	t.Run("GenerateMarkdown handles highlight with only note", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Note Only Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{
					Text: "",
					Note: "Just a note without highlight text",
				},
			},
		}

		markdown := GenerateMarkdown(book)

		assert.Contains(t, markdown, "> ")
		assert.Contains(t, markdown, "**Note:** Just a note without highlight text")
	})
}
