package exporters

import "github.com/mrlokans/assistant/internal/entities"

type BookExporter interface {
	Export(books []entities.Book) (ExportResult, error)
}

type ExportResult struct {
	BooksProcessed      int `json:"books_processed"`
	HighlightsProcessed int `json:"highlights_processed"`
	BooksFailed         int `json:"books_failed"`
	HighlightsFailed    int `json:"highlights_failed"`
}
