// Package exporters handles exporting books and highlights to markdown and database.
package exporters

import (
	"fmt"
	"log"

	"github.com/mrlokans/assistant/internal/entities"
)

// BookSaver persists books to the database.
type BookSaver interface {
	SaveBook(book *entities.Book) error
}

// DatabaseMarkdownExporter saves to the database and exports to markdown in one step.
type DatabaseMarkdownExporter struct {
	reader           BookReader
	saver            BookSaver
	markdownExporter *MarkdownExporter
}

// NewDatabaseMarkdownExporter creates an exporter backed by a book reader/saver and vault directory.
func NewDatabaseMarkdownExporter(reader BookReader, saver BookSaver, exportDir string) *DatabaseMarkdownExporter {
	return &DatabaseMarkdownExporter{
		reader:           reader,
		saver:            saver,
		markdownExporter: NewMarkdownExporter(exportDir),
	}
}

// Export saves books to the database and writes markdown files.
func (exporter *DatabaseMarkdownExporter) Export(books []entities.Book) (ExportResult, error) {
	result := ExportResult{}

	for i := range books {
		book := &books[i]
		err := exporter.saver.SaveBook(book)
		if err != nil {
			log.Printf("Failed to save book '%s' by %s to database: %v", book.Title, book.Author, err)
			result.BooksFailed++
			continue
		}
		result.BooksProcessed++
		result.HighlightsProcessed += len(book.Highlights)
		log.Printf("Successfully saved book '%s' by %s to database with ID %d", book.Title, book.Author, book.ID)
	}

	markdownResult, err := exporter.markdownExporter.Export(books)
	if err != nil {
		if err == ErrExportDirNotConfigured {
			log.Printf("Markdown export skipped: export directory not configured")
		} else {
			return result, fmt.Errorf("failed to export to markdown: %w", err)
		}
	} else {
		if markdownResult.BooksFailed > 0 {
			result.BooksFailed += markdownResult.BooksFailed
		}
		if markdownResult.HighlightsFailed > 0 {
			result.HighlightsFailed += markdownResult.HighlightsFailed
		}
	}

	log.Printf("Export completed: %d books processed, %d highlights processed, %d books failed, %d highlights failed",
		result.BooksProcessed, result.HighlightsProcessed, result.BooksFailed, result.HighlightsFailed)

	return result, nil
}

// GetAllBooks retrieves all books from the database.
func (exporter *DatabaseMarkdownExporter) GetAllBooks() ([]entities.Book, error) {
	return exporter.reader.GetAllBooks()
}

// GetBookByTitleAndAuthor retrieves a specific book from the database.
func (exporter *DatabaseMarkdownExporter) GetBookByTitleAndAuthor(title, author string) (*entities.Book, error) {
	return exporter.reader.GetBookByTitleAndAuthor(title, author)
}

// GetBookByID retrieves a book by its ID from the database.
func (exporter *DatabaseMarkdownExporter) GetBookByID(id uint) (*entities.Book, error) {
	return exporter.reader.GetBookByID(id)
}

// SearchBooks searches books by title (case-insensitive partial match).
func (exporter *DatabaseMarkdownExporter) SearchBooks(query string) ([]entities.Book, error) {
	return exporter.reader.SearchBooks(query)
}

// Compile-time interface implementation checks
var _ BookReader = (*DatabaseMarkdownExporter)(nil)
var _ BookExporter = (*DatabaseMarkdownExporter)(nil)
