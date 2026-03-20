package http

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/kindle"
)

const (
	maxKindleFileSize = 10 * 1024 * 1024 // 10 MB
)

// KindleImportController handles Kindle clippings file imports.
type KindleImportController struct {
	exporter     exporters.BookExporter
	auditService *audit.Service
}

// NewKindleImportController creates a Kindle import controller.
func NewKindleImportController(exporter exporters.BookExporter, auditService *audit.Service) *KindleImportController {
	return &KindleImportController{
		exporter:     exporter,
		auditService: auditService,
	}
}

// KindleImportResult summarises a Kindle import.
type KindleImportResult struct {
	Success            bool     `json:"success"`
	Error              string   `json:"error,omitempty"`
	BooksImported      int      `json:"books_imported"`
	HighlightsImported int      `json:"highlights_imported"`
	Errors             []string `json:"errors,omitempty"`
}

// Import processes an uploaded My Clippings.txt file.
func (c *KindleImportController) Import(ctx *gin.Context) {
	file, header, err := ctx.Request.FormFile("clippings_file")
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "kindle-import-result", &KindleImportResult{
			Success: false,
			Error:   "Clippings file not provided",
		})
		return
	}
	defer file.Close()

	// Check file size
	if header.Size > maxKindleFileSize {
		ctx.HTML(http.StatusBadRequest, "kindle-import-result", &KindleImportResult{
			Success: false,
			Error:   fmt.Sprintf("File too large (max %d MB)", maxKindleFileSize/(1024*1024)),
		})
		return
	}

	// Read file with size limit
	limitedReader := io.LimitReader(file, maxKindleFileSize+1)

	// Parse clippings
	parser := kindle.NewParser()
	books, err := parser.Parse(limitedReader)
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "kindle-import-result", &KindleImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse clippings: %v", err),
		})
		return
	}

	if len(books) == 0 {
		ctx.HTML(http.StatusOK, "kindle-import-result", &KindleImportResult{
			Success:            true,
			BooksImported:      0,
			HighlightsImported: 0,
			Errors:             []string{"No books with highlights found in the clippings file"},
		})
		return
	}

	// Stamp authenticated user on each book and its highlights
	userID := GetUserID(ctx)
	for i := range books {
		books[i].UserID = userID
		for j := range books[i].Highlights {
			books[i].Highlights[j].UserID = userID
		}
	}

	// Export to database (and optionally markdown)
	result, exportErr := c.exporter.Export(books)

	// Log the import event
	if c.auditService != nil {
		desc := fmt.Sprintf("Imported %d books with %d highlights from Kindle", result.BooksProcessed, result.HighlightsProcessed)
		c.auditService.LogImport(auth.GetUserID(ctx), "kindle", desc, result.BooksProcessed, result.HighlightsProcessed, exportErr)
	}

	if exportErr != nil {
		ctx.HTML(http.StatusInternalServerError, "kindle-import-result", &KindleImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to export: %v", exportErr),
		})
		return
	}

	ctx.HTML(http.StatusOK, "kindle-import-result", &KindleImportResult{
		Success:            true,
		BooksImported:      result.BooksProcessed,
		HighlightsImported: result.HighlightsProcessed,
	})
}
