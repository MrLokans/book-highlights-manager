package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/utils"
)

// MoonReaderHighlight represents a single highlight from MoonReader
type MoonReaderHighlight struct {
	ID             int64  `json:"id"`
	BookTitle      string `json:"book_title"`
	Filename       string `json:"filename"`
	HighlightColor string `json:"highlight_color"`
	TimeMs         int64  `json:"time"`
	Bookmark       string `json:"bookmark"`
	Note           string `json:"note"`
	Original       string `json:"original"`
	Underline      int    `json:"underline"`
	Strikethrough  int    `json:"strikethrough"`
}

// MoonReaderImportRequest is the request body for MoonReader import
type MoonReaderImportRequest struct {
	Highlights []MoonReaderHighlight `json:"highlights"`
}

// MoonReaderImportResponse is the response for MoonReader import
type MoonReaderImportResponse struct {
	BooksProcessed      int `json:"books_processed"`
	HighlightsProcessed int `json:"highlights_processed"`
	BooksFailed         int `json:"books_failed"`
	HighlightsFailed    int `json:"highlights_failed"`
}

// MoonReaderImportController handles MoonReader highlight imports
type MoonReaderImportController struct {
	Exporter *exporters.DatabaseMarkdownExporter
	Auditor  *audit.Auditor
}

// NewMoonReaderImportController creates a new MoonReaderImportController
func NewMoonReaderImportController(exporter *exporters.DatabaseMarkdownExporter, auditor *audit.Auditor) *MoonReaderImportController {
	return &MoonReaderImportController{
		Exporter: exporter,
		Auditor:  auditor,
	}
}

// Import handles POST /import/moonreader
func (controller *MoonReaderImportController) Import(c *gin.Context) {
	var req MoonReaderImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Highlights) == 0 {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "no highlights provided"})
		return
	}

	// Audit the request
	if controller.Auditor != nil {
		if _, err := controller.Auditor.SaveJSON(req); err != nil {
			// Log but don't fail the request
			c.Writer.Header().Set("X-Audit-Warning", "Failed to save audit log")
		}
	}

	// Convert MoonReader highlights to books
	books := moonReaderHighlightsToBooks(req.Highlights)

	// Export using the combined exporter
	result, exportError := controller.Exporter.Export(books)
	if exportError != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": exportError.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, MoonReaderImportResponse{
		BooksProcessed:      result.BooksProcessed,
		HighlightsProcessed: result.HighlightsProcessed,
		BooksFailed:         result.BooksFailed,
		HighlightsFailed:    result.HighlightsFailed,
	})
}

// moonReaderHighlightsToBooks converts MoonReader highlights to entity Books
func moonReaderHighlightsToBooks(highlights []MoonReaderHighlight) []entities.Book {
	// Group highlights by book title + author
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
		}

		bookMap[key].Highlights = append(bookMap[key].Highlights, highlight)
	}

	// Convert map to slice
	books := make([]entities.Book, 0, len(bookMap))
	for _, book := range bookMap {
		books = append(books, *book)
	}

	return books
}
