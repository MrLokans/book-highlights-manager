// Package exporters handles exporting books and highlights to markdown and database.
package exporters

import (
	"fmt"
	"log/slog"

	"github.com/mrlokans/assistant/internal/entities"
)

// BookSaver persists books to the database.
type BookSaver interface {
	SaveBook(book *entities.Book) error
}

// DatabaseMarkdownExporter saves to the database and exports to markdown in one step.
type DatabaseMarkdownExporter struct {
	saver            BookSaver
	markdownExporter *MarkdownExporter
}

// NewDatabaseMarkdownExporter creates an exporter backed by a book saver and vault directory.
func NewDatabaseMarkdownExporter(saver BookSaver, exportDir string) *DatabaseMarkdownExporter {
	return &DatabaseMarkdownExporter{
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
			slog.Error("Failed to save book to database", "title", book.Title, "author", book.Author, "error", err)
			result.BooksFailed++
			continue
		}
		result.BooksProcessed++
		result.HighlightsProcessed += len(book.Highlights)
		slog.Info("Saved book to database", "title", book.Title, "author", book.Author, "id", book.ID)
	}

	markdownResult, err := exporter.markdownExporter.Export(books)
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
		"books_failed", result.BooksFailed, "highlights_failed", result.HighlightsFailed)

	return result, nil
}

// Compile-time interface implementation check
var _ BookExporter = (*DatabaseMarkdownExporter)(nil)
