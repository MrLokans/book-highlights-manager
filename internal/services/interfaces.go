// Package services provides canonical interface and type definitions shared across the application.
package services

import "github.com/mrlokans/assistant/internal/exporters"

// BookReader provides read-only access to books and highlights.
type BookReader = exporters.BookReader

// BookExporter handles exporting books to storage (database + files).
type BookExporter = exporters.BookExporter

// ExportResult contains the outcome of an export operation.
type ExportResult = exporters.ExportResult

// ImportResult contains the outcome of an import operation.
type ImportResult struct {
	BooksProcessed      int
	HighlightsProcessed int
	BooksFailed         int
	HighlightsFailed    int
}
