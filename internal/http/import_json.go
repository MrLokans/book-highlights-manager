package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
)

type KoReaderSingleHighlight struct {
	// Current ISO 8601 timestamp
	// Time    time.Time `json:"time"`
	Sort    string `json:"sort"`
	Drawer  string `json:"drawer"`
	Chapter string `json:"chapter"`
	Text    string `json:"text"`
	Page    int    `json:"page"`
}

type KoReaderBookWithHighlights struct {
	Title   string                    `json:"title"`
	Author  string                    `json:"author"`
	File    string                    `json:"file"`
	Md5sum  string                    `json:"md5sum"`
	Entries []KoReaderSingleHighlight `json:"entries"`
}

type KoReaderImportRequest struct {
	Version string `json:"version"`
	// CreatedOn time.Time                `json:"created_on"`
	Documents []KoReaderBookWithHighlights `json:"documents"`
}

type KoReaderImportResponse struct {
	BooksProcessed      int `json:"books_processed"`
	HighlightsProcessed int `json:"highlights_processed"`
	BooksFailed         int `json:"books_failed"`
	HighlightsFailed    int `json:"highlights_failed"`
}

func AsHighlights(koHighlights []KoReaderSingleHighlight) []entities.Highlight {
	highlights := make([]entities.Highlight, len(koHighlights))
	for i, highlight := range koHighlights {
		highlights[i] = entities.Highlight{
			Text: highlight.Text,
			Page: highlight.Page,
		}
	}
	return highlights
}

func AsBooks(req KoReaderImportRequest) []entities.Book {
	books := make([]entities.Book, len(req.Documents))
	for i, doc := range req.Documents {
		books[i] = entities.Book{
			Title:      doc.Title,
			Author:     doc.Author,
			File:       doc.File,
			Highlights: AsHighlights(doc.Entries),
			Source:     entities.Source{Name: "koreader"},
		}
	}
	return books
}

func AsResponse(result exporters.ExportResult) KoReaderImportResponse {
	return KoReaderImportResponse{
		BooksProcessed:      result.BooksProcessed,
		HighlightsProcessed: result.HighlightsProcessed,
		BooksFailed:         result.BooksFailed,
		HighlightsFailed:    result.HighlightsFailed,
	}
}

type KoReaderImportController struct {
	Exporter exporters.BookExporter
}

func (controller KoReaderImportController) Import(c *gin.Context) {
	var req KoReaderImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, exportError := controller.Exporter.Export(AsBooks(req))
	if exportError != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": exportError.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, AsResponse(result))
}

func NewKoReaderImportController(exporter exporters.BookExporter) KoReaderImportController {
	return KoReaderImportController{
		Exporter: exporter,
	}
}
