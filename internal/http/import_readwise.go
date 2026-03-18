package http

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
)

// ReadwiseSingleHighlight is the JSON payload for a single Readwise highlight.
type ReadwiseSingleHighlight struct {
	Text          string `json:"text"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	SourceType    string `json:"source_type"`
	Category      string `json:"category"`
	Note          string `json:"note"`
	Page          int    `json:"location"`
	LocationType  string `json:"location_type"`
	HighlightedAt string `json:"highlighted_at"`
	ID            string `json:"id"`
}

// GroupKey returns a unique key for grouping highlights by book.
func (highlight ReadwiseSingleHighlight) GroupKey() string {
	return highlight.Author + highlight.Title
}

// ReadwiseImportRequest is the JSON body for Readwise imports.
type ReadwiseImportRequest struct {
	Highlights []ReadwiseSingleHighlight `json:"highlights"`
}

// ReadwiseImportResponse is the JSON response for Readwise imports.
type ReadwiseImportResponse struct {
	BooksProcessed      int `json:"books_processed"`
	HighlightsProcessed int `json:"highlights_processed"`
	BooksFailed         int `json:"books_failed"`
	HighlightsFailed    int `json:"highlights_failed"`
}

func asBooks(req ReadwiseImportRequest) []entities.Book {
	// Go though all highlights and group them by author and title, then create a book for each group
	books := make([]entities.Book, 0)
	bookMap := make(map[string]entities.Book)
	for _, highlight := range req.Highlights {
		key := highlight.GroupKey()
		book, ok := bookMap[key]
		if !ok {
			book = entities.Book{
				Title:      highlight.Title,
				Author:     highlight.Author,
				Highlights: make([]entities.Highlight, 0),
				Source:     entities.Source{Name: "readwise"},
			}
			bookMap[key] = book
		}
		newHighlight := entities.Highlight{
			Text:          highlight.Text,
			LocationValue: highlight.Page,
		}
		book.Highlights = append(book.Highlights, newHighlight)
		bookMap[key] = book
	}
	for _, book := range bookMap {
		books = append(books, book)
	}
	return books
}

func asResponse(result exporters.ExportResult) ReadwiseImportResponse {
	return ReadwiseImportResponse(result)
}

// ReadwiseAPIImportController handles Readwise API highlight imports.
type ReadwiseAPIImportController struct {
	Exporter     exporters.BookExporter
	Token        string
	AuditService *audit.Service
}

// Import processes a batch of Readwise highlights.
func (controller ReadwiseAPIImportController) Import(c *gin.Context) {
	// Check if Readwise integration is configured
	if controller.Token == "" {
		c.IndentedJSON(http.StatusNotImplemented, gin.H{
			"error":   "Readwise integration not configured",
			"message": "Set READWISE_TOKEN environment variable to enable Readwise imports",
		})
		return
	}

	// Extract auth token from header
	token := c.GetHeader("Authorization")

	if token == "" || len(token) < 6 {
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Remove the 'Token ' prefix
	token = token[6:]

	if token != controller.Token {
		c.IndentedJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req ReadwiseImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	books := asBooks(req)
	result, exportError := controller.Exporter.Export(books)

	// Log the import event
	if controller.AuditService != nil {
		desc := fmt.Sprintf("Imported %d books with %d highlights from Readwise API", result.BooksProcessed, result.HighlightsProcessed)
		controller.AuditService.LogImport(auth.GetUserID(c), "readwise_api", desc, result.BooksProcessed, result.HighlightsProcessed, exportError)
	}
	if exportError != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": exportError.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, asResponse(result))
}

// NewReadwiseAPIImportController creates a Readwise import controller.
func NewReadwiseAPIImportController(exporter exporters.BookExporter, token string, auditService *audit.Service) ReadwiseAPIImportController {
	return ReadwiseAPIImportController{Exporter: exporter, Token: token, AuditService: auditService}
}
