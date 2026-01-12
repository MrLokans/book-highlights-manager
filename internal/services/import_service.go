package services

import (
	"fmt"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/utils"
)

// HighlightInput represents a generic highlight from any source.
// This is the common format that all importers should convert to.
type HighlightInput struct {
	// Book identification
	BookTitle  string
	BookAuthor string
	FilePath   string

	// Highlight content
	Text string
	Note string

	// Location information
	LocationType  entities.LocationType
	LocationValue int
	Chapter       string

	// Styling
	Color string
	Style entities.HighlightStyle

	// Timestamps
	HighlightedAt time.Time

	// External reference
	ExternalID string
}

// ReadwiseHighlightInput represents highlight data from Readwise API.
type ReadwiseHighlightInput struct {
	Text          string
	Title         string
	Author        string
	SourceType    string
	Category      string
	Note          string
	Location      int
	LocationType  string
	HighlightedAt string
	ID            string
}

// MoonReaderHighlightInput represents highlight data from MoonReader.
type MoonReaderHighlightInput struct {
	ID             int64
	BookTitle      string
	Filename       string
	HighlightColor string
	TimeMs         int64
	Bookmark       string
	Note           string
	Original       string
	Underline      int
	Strikethrough  int
}

// ImportService handles the business logic for importing highlights
// from various sources and converting them to the internal format.
type ImportService struct {
	exporter BookExporter
}

// NewImportService creates a new ImportService.
func NewImportService(exporter BookExporter) *ImportService {
	return &ImportService{
		exporter: exporter,
	}
}

// ImportReadwiseHighlights converts Readwise highlights to books and exports them.
func (s *ImportService) ImportReadwiseHighlights(highlights []ReadwiseHighlightInput) (ImportResult, error) {
	books := s.readwiseToBooks(highlights)
	exportResult, err := s.exporter.Export(books)
	if err != nil {
		return ImportResult{}, fmt.Errorf("failed to export: %w", err)
	}
	return ImportResult(exportResult), nil
}

// ImportMoonReaderHighlights converts MoonReader highlights to books and exports them.
func (s *ImportService) ImportMoonReaderHighlights(highlights []MoonReaderHighlightInput) (ImportResult, error) {
	books := s.moonReaderToBooks(highlights)
	exportResult, err := s.exporter.Export(books)
	if err != nil {
		return ImportResult{}, fmt.Errorf("failed to export: %w", err)
	}
	return ImportResult(exportResult), nil
}

// ImportGenericHighlights converts generic highlight inputs to books and exports them.
func (s *ImportService) ImportGenericHighlights(highlights []HighlightInput) (ImportResult, error) {
	books := s.genericToBooks(highlights)
	exportResult, err := s.exporter.Export(books)
	if err != nil {
		return ImportResult{}, fmt.Errorf("failed to export: %w", err)
	}
	return ImportResult(exportResult), nil
}

// readwiseToBooks groups Readwise highlights by book and converts to entities.
func (s *ImportService) readwiseToBooks(highlights []ReadwiseHighlightInput) []entities.Book {
	bookMap := make(map[string]*entities.Book)

	for _, h := range highlights {
		key := h.Author + "|" + h.Title
		if _, exists := bookMap[key]; !exists {
			bookMap[key] = &entities.Book{
				Title:      h.Title,
				Author:     h.Author,
				Highlights: []entities.Highlight{},
				Source:     entities.Source{Name: "readwise"},
			}
		}

		highlight := entities.Highlight{
			Text:          h.Text,
			Note:          h.Note,
			LocationType:  entities.LocationTypeLocation,
			LocationValue: h.Location,
			Source:        entities.Source{Name: "readwise"},
			ExternalID:    h.ID,
			// Legacy field support - parse time if needed
			Time: h.HighlightedAt,
		}

		// Parse highlighted_at timestamp if provided
		if h.HighlightedAt != "" {
			if t, err := time.Parse(time.RFC3339, h.HighlightedAt); err == nil {
				highlight.HighlightedAt = t
			}
		}

		bookMap[key].Highlights = append(bookMap[key].Highlights, highlight)
	}

	books := make([]entities.Book, 0, len(bookMap))
	for _, book := range bookMap {
		books = append(books, *book)
	}
	return books
}

// moonReaderToBooks groups MoonReader highlights by book and converts to entities.
func (s *ImportService) moonReaderToBooks(highlights []MoonReaderHighlightInput) []entities.Book {
	bookMap := make(map[string]*entities.Book)

	for _, h := range highlights {
		// Extract author from filename
		author := utils.ExtractAuthorFromFilename(h.Filename, h.BookTitle)
		key := h.BookTitle + "|" + author

		if _, exists := bookMap[key]; !exists {
			bookMap[key] = &entities.Book{
				Title:      h.BookTitle,
				Author:     author,
				FilePath:   h.Filename,
				Highlights: []entities.Highlight{},
				Source:     entities.Source{Name: "moonreader"},
			}
		}

		// Determine highlight style
		style := entities.HighlightStyleHighlight
		if h.Underline != 0 {
			style = entities.HighlightStyleUnderline
		} else if h.Strikethrough != 0 {
			style = entities.HighlightStyleStrikethrough
		}

		// Get text (prefer original over note)
		text := h.Original
		noteText := h.Note
		if text == "" {
			text = h.Note
			noteText = ""
		}

		// Convert color
		color, _ := utils.InternalColorToHexARGB(h.HighlightColor)

		highlight := entities.Highlight{
			Text:          text,
			Note:          noteText,
			Color:         color,
			Style:         style,
			HighlightedAt: time.UnixMilli(h.TimeMs),
			Chapter:       h.Bookmark,
			LocationType:  entities.LocationTypeNone,
			ExternalID:    fmt.Sprintf("%d", h.ID),
			Source:        entities.Source{Name: "moonreader"},
		}

		bookMap[key].Highlights = append(bookMap[key].Highlights, highlight)
	}

	books := make([]entities.Book, 0, len(bookMap))
	for _, book := range bookMap {
		books = append(books, *book)
	}
	return books
}

// genericToBooks groups generic highlights by book and converts to entities.
func (s *ImportService) genericToBooks(highlights []HighlightInput) []entities.Book {
	bookMap := make(map[string]*entities.Book)

	for _, h := range highlights {
		key := h.BookTitle + "|" + h.BookAuthor

		if _, exists := bookMap[key]; !exists {
			bookMap[key] = &entities.Book{
				Title:      h.BookTitle,
				Author:     h.BookAuthor,
				FilePath:   h.FilePath,
				Highlights: []entities.Highlight{},
			}
		}

		highlight := entities.Highlight{
			Text:          h.Text,
			Note:          h.Note,
			Color:         h.Color,
			Style:         h.Style,
			HighlightedAt: h.HighlightedAt,
			Chapter:       h.Chapter,
			LocationType:  h.LocationType,
			LocationValue: h.LocationValue,
			ExternalID:    h.ExternalID,
		}

		bookMap[key].Highlights = append(bookMap[key].Highlights, highlight)
	}

	books := make([]entities.Book, 0, len(bookMap))
	for _, book := range bookMap {
		books = append(books, *book)
	}
	return books
}
