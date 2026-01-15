package importers

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

// ReadwiseCSVRow represents a single row from a Readwise CSV export.
type ReadwiseCSVRow struct {
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

// ReadwiseCSVConverter converts Readwise CSV export data to the common format.
type ReadwiseCSVConverter struct {
	Rows []ReadwiseCSVRow
}

// NewReadwiseCSVConverter creates a converter for Readwise CSV data.
func NewReadwiseCSVConverter(rows []ReadwiseCSVRow) *ReadwiseCSVConverter {
	return &ReadwiseCSVConverter{Rows: rows}
}

// Convert implements Converter interface.
func (c *ReadwiseCSVConverter) Convert() ([]RawHighlight, Source) {
	highlights := make([]RawHighlight, 0, len(c.Rows))

	for _, row := range c.Rows {
		h := RawHighlight{
			BookTitle:  row.BookTitle,
			BookAuthor: row.BookAuthor,
			Text:       row.Highlight,
			Note:       row.Note,
			Color:      normalizeColor(row.Color),
		}

		// Parse location type
		h.LocationType = parseLocationType(row.LocationType)

		// Parse location value
		if row.Location != "" {
			if loc, err := strconv.Atoi(row.Location); err == nil {
				h.LocationValue = loc
			}
		}

		// Parse highlighted at timestamp
		if row.HighlightedAt != "" {
			if t, err := parseReadwiseTimestamp(row.HighlightedAt); err == nil {
				h.HighlightedAt = t.Format(time.RFC3339)
			}
		}

		highlights = append(highlights, h)
	}

	return highlights, Source{Name: "readwise"}
}

// ParseReadwiseCSV parses a Readwise CSV export file.
// Returns the parsed rows, any parse errors encountered, and a fatal error if parsing fails completely.
func ParseReadwiseCSV(r io.Reader) ([]ReadwiseCSVRow, []string, error) {
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

	var rows []ReadwiseCSVRow
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

		row := ReadwiseCSVRow{}

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
		"yellow": "#FFFF00",
		"blue":   "#0000FF",
		"pink":   "#FFC0CB",
		"orange": "#FFA500",
		"green":  "#00FF00",
		"purple": "#800080",
		"red":    "#FF0000",
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

// Compile-time interface check
var _ Converter = (*ReadwiseCSVConverter)(nil)
