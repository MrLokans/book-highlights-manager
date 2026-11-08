// Package exporters handles exporting books and highlights to markdown and database.
package exporters

import (
	"fmt"
	"log/slog"

	booksdb "github.com/mrlokans/assistant/internal/database/books"

	"github.com/mrlokans/assistant/internal/entities"
)

// DatabaseMarkdownExporter saves to the database and exports to markdown in one step.
type DatabaseMarkdownExporter struct {
	saver            *booksdb.Repository
	markdownExporter *MarkdownExporter
}

// NewDatabaseMarkdownExporter creates an exporter backed by a book saver and vault directory.
func NewDatabaseMarkdownExporter(saver *booksdb.Repository, exportDir string) *DatabaseMarkdownExporter {
	return &DatabaseMarkdownExporter{
		saver:            saver,
		markdownExporter: NewMarkdownExporter(exportDir),
	}
}

// Export saves books to the database and writes markdown files.
func (exporter *DatabaseMarkdownExporter) Export(books []entities.Book) (ExportResult, error) {
	result := ExportResult{}
	saved := make([]entities.Book, 0, len(books))

	for i := range books {
		book := &books[i]
		added, err := exporter.saver.ImportBook(book)
		if err != nil {
			slog.Error("Failed to save book to database", "title", book.Title, "author", book.Author, "error", err)
			result.BooksFailed++
			continue
		}
		// SaveBook leaves ID unset for permanently deleted books it skipped.
		if book.ID == 0 {
			continue
		}
		saved = append(saved, *book)
		result.BooksProcessed++
		result.HighlightsProcessed += len(book.Highlights)
		result.HighlightsAdded += added
		slog.Info("Saved book to database", "title", book.Title, "author", book.Author, "id", book.ID)
	}

	markdownResult, err := exporter.markdownExporter.Export(saved)
	if err != nil {
		if err == ErrExportDirNotConfigured {
			slog.Info("Markdown export skipped: export directory not configured")
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

	slog.Info("Export completed",
		"books_processed", result.BooksProcessed, "highlights_processed", result.HighlightsProcessed,
		"highlights_added", result.HighlightsAdded, "books_failed", result.BooksFailed, "highlights_failed", result.HighlightsFailed)

	return result, nil
}

// Compile-time interface implementation check
var _ BookExporter = (*DatabaseMarkdownExporter)(nil)
