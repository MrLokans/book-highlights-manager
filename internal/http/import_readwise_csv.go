package http

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
)

type ReadwiseCSVImportController struct {
	exporter     exporters.BookExporter
	auditService *audit.Service
}

func NewReadwiseCSVImportController(exporter exporters.BookExporter, auditService *audit.Service) *ReadwiseCSVImportController {
	return &ReadwiseCSVImportController{
		exporter:     exporter,
		auditService: auditService,
	}
}

type ReadwiseCSVImportResult struct {
	Success           bool     `json:"success"`
	Error             string   `json:"error,omitempty"`
	TotalRows         int      `json:"total_rows"`
	BooksImported     int      `json:"books_imported"`
	HighlightsImported int      `json:"highlights_imported"`
	Errors            []string `json:"errors,omitempty"`
}

type readwiseCSVRow struct {
	Highlight     string
	BookTitle     string
	BookAuthor    string
	AmazonBookID  string
	Note          string
	Color         string
	Tags          string
	LocationType  string
	Location      string
	HighlightedAt string
	DocumentTags  string
}

func (c *ReadwiseCSVImportController) Import(ctx *gin.Context) {
	file, _, err := ctx.Request.FormFile("csv_file")
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "readwise-csv-import-result", &ReadwiseCSVImportResult{
			Success: false,
			Error:   "No CSV file provided",
		})
		return
	}
	defer file.Close()

	// Parse CSV
	rows, parseErrors, err := parseReadwiseCSV(file)
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "readwise-csv-import-result", &ReadwiseCSVImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse CSV: %v", err),
		})
		return
	}

	result := &ReadwiseCSVImportResult{
		Success:   true,
		TotalRows: len(rows),
		Errors:    parseErrors,
	}

	// Group highlights by book
	books := groupHighlightsByBook(rows)
	result.BooksImported = len(books)

	// Count total highlights
	for _, book := range books {
		result.HighlightsImported += len(book.Highlights)
	}

	// Export to database and markdown
	_, exportErr := c.exporter.Export(books)

	// Log the import event
	if c.auditService != nil {
		desc := fmt.Sprintf("Imported %d books with %d highlights from Readwise CSV", result.BooksImported, result.HighlightsImported)
		c.auditService.LogImport(DefaultUserID, "readwise_csv", desc, result.BooksImported, result.HighlightsImported, exportErr)
	}

	if exportErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Export error: %v", exportErr))
	}

	ctx.HTML(http.StatusOK, "readwise-csv-import-result", result)
}

func parseReadwiseCSV(r io.Reader) ([]readwiseCSVRow, []string, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read header row
	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Build header index map
	headerIndex := make(map[string]int)
	for i, h := range header {
		headerIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}

	// Validate required headers
	requiredHeaders := []string{"highlight", "book title", "book author"}
	for _, h := range requiredHeaders {
		if _, ok := headerIndex[h]; !ok {
			return nil, nil, fmt.Errorf("missing required header: %s", h)
		}
	}

	var rows []readwiseCSVRow
	var errors []string
	lineNum := 1 // Start at 1 because we already read the header

	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("Line %d: %v", lineNum, err))
			continue
		}

		row := readwiseCSVRow{}

		// Safely get values from record using header index
		row.Highlight = getCSVValue(record, headerIndex, "highlight")
		row.BookTitle = getCSVValue(record, headerIndex, "book title")
		row.BookAuthor = getCSVValue(record, headerIndex, "book author")
		row.AmazonBookID = getCSVValue(record, headerIndex, "amazon book id")
		row.Note = getCSVValue(record, headerIndex, "note")
		row.Color = getCSVValue(record, headerIndex, "color")
		row.Tags = getCSVValue(record, headerIndex, "tags")
		row.LocationType = getCSVValue(record, headerIndex, "location type")
		row.Location = getCSVValue(record, headerIndex, "location")
		row.HighlightedAt = getCSVValue(record, headerIndex, "highlighted at")
		row.DocumentTags = getCSVValue(record, headerIndex, "document tags")

		// Skip rows without highlight text or book title
		if row.Highlight == "" || row.BookTitle == "" {
			errors = append(errors, fmt.Sprintf("Line %d: skipped - missing highlight or book title", lineNum))
			continue
		}

		rows = append(rows, row)
	}

	return rows, errors, nil
}

func getCSVValue(record []string, headerIndex map[string]int, header string) string {
	if idx, ok := headerIndex[header]; ok && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}

func groupHighlightsByBook(rows []readwiseCSVRow) []entities.Book {
	// Use a map to group highlights by book key (title + author)
	bookMap := make(map[string]*entities.Book)

	for _, row := range rows {
		bookKey := row.BookTitle + "|" + row.BookAuthor

		book, exists := bookMap[bookKey]
		if !exists {
			book = &entities.Book{
				Title:  row.BookTitle,
				Author: row.BookAuthor,
				ASIN:   row.AmazonBookID,
			}
			bookMap[bookKey] = book
		}

		// Parse highlight
		highlight := convertRowToHighlight(row)
		book.Highlights = append(book.Highlights, highlight)
	}

	// Convert map to slice
	books := make([]entities.Book, 0, len(bookMap))
	for _, book := range bookMap {
		books = append(books, *book)
	}

	return books
}

func convertRowToHighlight(row readwiseCSVRow) entities.Highlight {
	highlight := entities.Highlight{
		Text:  row.Highlight,
		Note:  row.Note,
		Color: normalizeColor(row.Color),
	}

	// Parse location type
	highlight.LocationType = parseLocationType(row.LocationType)

	// Parse location value
	if row.Location != "" {
		if loc, err := strconv.Atoi(row.Location); err == nil {
			highlight.LocationValue = loc
		}
	}

	// Parse highlighted at timestamp
	if row.HighlightedAt != "" {
		if t, err := parseReadwiseTimestamp(row.HighlightedAt); err == nil {
			highlight.HighlightedAt = t
		}
	}

	return highlight
}

func parseLocationType(locType string) entities.LocationType {
	switch strings.ToLower(locType) {
	case "page":
		return entities.LocationTypePage
	case "location":
		return entities.LocationTypeLocation
	case "order":
		return entities.LocationTypePosition
	case "time":
		return entities.LocationTypeTime
	default:
		return entities.LocationTypeNone
	}
}

func normalizeColor(color string) string {
	colorMap := map[string]string{
		"yellow":  "#FFFF00",
		"blue":    "#0000FF",
		"pink":    "#FFC0CB",
		"orange":  "#FFA500",
		"green":   "#00FF00",
		"purple":  "#800080",
		"red":     "#FF0000",
	}

	if hex, ok := colorMap[strings.ToLower(color)]; ok {
		return hex
	}

	// Return as-is if already hex or unknown
	return color
}

func parseReadwiseTimestamp(ts string) (time.Time, error) {
	// Try various formats
	formats := []string{
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05+00:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, ts); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", ts)
}
