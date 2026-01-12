package exporters

import (
	"fmt"
	"log"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
)

type DatabaseMarkdownExporter struct {
	db               *database.Database
	markdownExporter *MarkdownExporter
}

func NewDatabaseMarkdownExporter(db *database.Database, vaultDir string, exportPath string) *DatabaseMarkdownExporter {
	return &DatabaseMarkdownExporter{
		db:               db,
		markdownExporter: NewMarkdownExporter(vaultDir, exportPath),
	}
}

func (exporter *DatabaseMarkdownExporter) Export(books []entities.Book) (ExportResult, error) {
	result := ExportResult{}

	// First, save all books to the database
	for i := range books {
		book := &books[i]
		err := exporter.db.SaveBook(book)
		if err != nil {
			log.Printf("Failed to save book '%s' by %s to database: %v", book.Title, book.Author, err)
			result.BooksFailed++
			continue
		}
		result.BooksProcessed++
		result.HighlightsProcessed += len(book.Highlights)
		log.Printf("Successfully saved book '%s' by %s to database with ID %d", book.Title, book.Author, book.ID)
	}

	// Then export to markdown files
	markdownResult, err := exporter.markdownExporter.Export(books)
	if err != nil {
		return result, fmt.Errorf("failed to export to markdown: %w", err)
	}

	// Combine results (database save counts are already in result, markdown export should match)
	if markdownResult.BooksFailed > 0 {
		result.BooksFailed += markdownResult.BooksFailed
	}
	if markdownResult.HighlightsFailed > 0 {
		result.HighlightsFailed += markdownResult.HighlightsFailed
	}

	log.Printf("Export completed: %d books processed, %d highlights processed, %d books failed, %d highlights failed",
		result.BooksProcessed, result.HighlightsProcessed, result.BooksFailed, result.HighlightsFailed)

	return result, nil
}

// GetAllBooks retrieves all books from the database.
// Implements BookReader interface.
func (exporter *DatabaseMarkdownExporter) GetAllBooks() ([]entities.Book, error) {
	return exporter.db.GetAllBooks()
}

// GetBookByTitleAndAuthor retrieves a specific book from the database.
// Implements BookReader interface.
func (exporter *DatabaseMarkdownExporter) GetBookByTitleAndAuthor(title, author string) (*entities.Book, error) {
	return exporter.db.GetBookByTitleAndAuthor(title, author)
}

// GetBookByID retrieves a book by its ID from the database.
// Implements BookReader interface.
func (exporter *DatabaseMarkdownExporter) GetBookByID(id uint) (*entities.Book, error) {
	return exporter.db.GetBookByID(id)
}

// SearchBooks searches books by title (case-insensitive partial match).
// Implements BookReader interface.
func (exporter *DatabaseMarkdownExporter) SearchBooks(query string) ([]entities.Book, error) {
	return exporter.db.SearchBooks(query)
}

// Compile-time interface implementation checks
var _ BookReader = (*DatabaseMarkdownExporter)(nil)
var _ BookExporter = (*DatabaseMarkdownExporter)(nil)
