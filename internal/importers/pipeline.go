package importers

import (
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/services"
)

// RawHighlight represents a highlight from any import source.
// Each import source implements a converter that transforms its
// native format into this common representation.
type RawHighlight struct {
	BookTitle     string
	BookAuthor    string
	Text          string
	Note          string
	Page          int
	LocationType  entities.LocationType
	LocationValue int
	Chapter       string
	Color         string
	Style         entities.HighlightStyle
	HighlightedAt string
	ExternalID    string
	FilePath      string
}

// GroupKey returns a unique identifier for grouping highlights by book.
func (h RawHighlight) GroupKey() string {
	return h.BookAuthor + "|" + h.BookTitle
}

// Source provides metadata about the import source for each book.
type Source struct {
	Name     string
	FilePath string
}

// Converter transforms raw highlights into entities ready for export.
// Each import source implements this interface.
//
// Implementations:
//   - ReadwiseConverter (readwise.go) - Readwise API JSON format
//   - ReadwiseCSVConverter (readwise_csv.go) - Readwise CSV export format
//   - MoonReaderConverter (moonreader.go) - Moon+ Reader JSON format
//
// Adding a new import source:
//  1. Create a new file (e.g., kobo.go)
//  2. Define your source-specific highlight struct
//  3. Implement Converter interface
//  4. Use Pipeline.Import() in your HTTP handler
type Converter interface {
	// Convert transforms raw data from the import source into RawHighlights.
	// Returns the highlights and the source metadata.
	Convert() ([]RawHighlight, Source)
}

// Exporter persists books to storage.
type Exporter interface {
	Export(books []entities.Book) (services.ExportResult, error)
}

// Pipeline handles the common import workflow:
// parse → group by book → deduplicate → save.
//
// This eliminates duplication across import handlers by providing
// a single point for the grouping and export logic.
type Pipeline struct {
	exporter Exporter
}

// NewPipeline creates a new import pipeline with the given exporter.
func NewPipeline(exporter Exporter) *Pipeline {
	return &Pipeline{exporter: exporter}
}

// Import processes highlights from a converter and exports them.
// This is the main entry point for all import operations.
func (p *Pipeline) Import(converter Converter) (services.ImportResult, error) {
	highlights, source := converter.Convert()

	if len(highlights) == 0 {
		return services.ImportResult{}, nil
	}

	books := groupHighlightsByBook(highlights, source)

	exportResult, err := p.exporter.Export(books)
	if err != nil {
		return services.ImportResult{}, err
	}

	return services.ImportResult(exportResult), nil
}

// ImportBooks directly exports pre-grouped books.
// Use this when the source already provides book-level grouping (e.g., Apple Books, Kindle).
func (p *Pipeline) ImportBooks(books []entities.Book) (services.ImportResult, error) {
	if len(books) == 0 {
		return services.ImportResult{}, nil
	}

	exportResult, err := p.exporter.Export(books)
	if err != nil {
		return services.ImportResult{}, err
	}

	return services.ImportResult(exportResult), nil
}

// groupHighlightsByBook groups raw highlights by book (title + author).
func groupHighlightsByBook(highlights []RawHighlight, source Source) []entities.Book {
	bookMap := make(map[string]*entities.Book)

	for _, h := range highlights {
		key := h.GroupKey()

		book, exists := bookMap[key]
		if !exists {
			book = &entities.Book{
				Title:    h.BookTitle,
				Author:   h.BookAuthor,
				FilePath: h.FilePath,
				Source:   entities.Source{Name: source.Name},
			}
			bookMap[key] = book
		}

		highlight := entities.Highlight{
			Text:          h.Text,
			Note:          h.Note,
			Page:          h.Page,
			LocationType:  h.LocationType,
			LocationValue: h.LocationValue,
			Chapter:       h.Chapter,
			Color:         h.Color,
			Style:         h.Style,
			Time:          h.HighlightedAt,
			ExternalID:    h.ExternalID,
		}

		book.Highlights = append(book.Highlights, highlight)
	}

	books := make([]entities.Book, 0, len(bookMap))
	for _, book := range bookMap {
		books = append(books, *book)
	}

	return books
}
