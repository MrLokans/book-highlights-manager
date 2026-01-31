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
	Id            string `json:"id"`
}

func (highlight ReadwiseSingleHighlight) GroupKey() string {
	return highlight.Author + highlight.Title
}

type ReadwiseImportRequest struct {
	Highlights []ReadwiseSingleHighlight `json:"highlights"`
}

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
			Time: highlight.HighlightedAt,
			Text: highlight.Text,
			Page: highlight.Page,
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

type ReadwiseAPIImportController struct {
	Exporter     exporters.BookExporter
	Token        string
	AuditService *audit.Service
}

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

func NewReadwiseAPIImportController(exporter exporters.BookExporter, token string, auditService *audit.Service) ReadwiseAPIImportController {
	return ReadwiseAPIImportController{Exporter: exporter, Token: token, AuditService: auditService}
}
