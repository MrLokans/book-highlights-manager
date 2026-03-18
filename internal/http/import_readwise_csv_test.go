package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLocationType(t *testing.T) {
	tests := []struct {
		input    string
		expected entities.LocationType
	}{
		{"page", entities.LocationTypePage},
		{"Page", entities.LocationTypePage},
		{"PAGE", entities.LocationTypePage},
		{"location", entities.LocationTypeLocation},
		{"order", entities.LocationTypePosition},
		{"time", entities.LocationTypeTime},
		{"unknown", entities.LocationTypeNone},
		{"", entities.LocationTypeNone},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, parseLocationType(tt.input))
		})
	}
}

func TestNormalizeColor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"yellow", "#FFFF00"},
		{"Yellow", "#FFFF00"},
		{"blue", "#0000FF"},
		{"pink", "#FFC0CB"},
		{"orange", "#FFA500"},
		{"green", "#00FF00"},
		{"purple", "#800080"},
		{"red", "#FF0000"},
		{"#FF0000", "#FF0000"},
		{"custom", "custom"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeColor(tt.input))
		})
	}
}

func TestParseReadwiseTimestamp(t *testing.T) {
	t.Run("parses ISO date", func(t *testing.T) {
		ts, err := parseReadwiseTimestamp("2024-01-15")
		require.NoError(t, err)
		assert.Equal(t, 2024, ts.Year())
		assert.Equal(t, 1, int(ts.Month()))
		assert.Equal(t, 15, ts.Day())
	})

	t.Run("parses datetime with timezone", func(t *testing.T) {
		ts, err := parseReadwiseTimestamp("2024-01-15 14:30:00+00:00")
		require.NoError(t, err)
		assert.Equal(t, 14, ts.Hour())
	})

	t.Run("parses ISO 8601", func(t *testing.T) {
		ts, err := parseReadwiseTimestamp("2024-01-15T14:30:00Z")
		require.NoError(t, err)
		assert.Equal(t, 14, ts.Hour())
	})

	t.Run("returns error for invalid", func(t *testing.T) {
		_, err := parseReadwiseTimestamp("not-a-date")
		assert.Error(t, err)
	})
}

func TestGetCSVValue(t *testing.T) {
	record := []string{"title", "author", "text"}
	headerIndex := map[string]int{"Title": 0, "Author": 1, "Highlight": 2}

	assert.Equal(t, "title", getCSVValue(record, headerIndex, "Title"))
	assert.Equal(t, "author", getCSVValue(record, headerIndex, "Author"))
	assert.Equal(t, "", getCSVValue(record, headerIndex, "Missing"))
	assert.Equal(t, "", getCSVValue([]string{}, headerIndex, "Title"))
}

func TestConvertRowToHighlight(t *testing.T) {
	row := readwiseCSVRow{
		Highlight:     "Some text",
		Note:          "My note",
		Color:         "yellow",
		LocationType:  "page",
		Location:      "42",
		HighlightedAt: "2024-01-15",
	}

	h := convertRowToHighlight(row)

	assert.Equal(t, "Some text", h.Text)
	assert.Equal(t, "My note", h.Note)
	assert.Equal(t, "#FFFF00", h.Color)
	assert.Equal(t, entities.LocationTypePage, h.LocationType)
	assert.Equal(t, 42, h.LocationValue)
	assert.Equal(t, 2024, h.HighlightedAt.Year())
}

func TestGroupHighlightsByBook(t *testing.T) {
	rows := []readwiseCSVRow{
		{BookTitle: "Book A", BookAuthor: "Author 1", Highlight: "h1"},
		{BookTitle: "Book A", BookAuthor: "Author 1", Highlight: "h2"},
		{BookTitle: "Book B", BookAuthor: "Author 2", Highlight: "h3", AmazonBookID: "B001"},
	}

	books := groupHighlightsByBook(rows)

	assert.Len(t, books, 2)

	bookMap := make(map[string]int)
	for _, b := range books {
		bookMap[b.Title] = len(b.Highlights)
	}
	assert.Equal(t, 2, bookMap["Book A"])
	assert.Equal(t, 1, bookMap["Book B"])
}

func TestReadwiseCSVImportController_Import_NoFile(t *testing.T) {
	exporter := &mockExporterForKindle{}
	controller := NewReadwiseCSVImportController(exporter, nil)

	router := newTestRouter(t)
	router.POST("/settings/readwise/import-csv", controller.Import)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/readwise/import-csv", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:readwise-csv-import-result")
}

func TestReadwiseCSVImportController_Import_ValidCSV(t *testing.T) {
	exporter := &mockExporterForKindle{result: exporters.ExportResult{BooksProcessed: 1, HighlightsProcessed: 1}}
	controller := NewReadwiseCSVImportController(exporter, nil)

	router := newTestRouter(t)
	router.POST("/settings/readwise/import-csv", controller.Import)

	csvContent := "Highlight,Book Title,Book Author,Note,Color\n\"Some highlight\",\"Test Book\",\"Author\",\"\",\"yellow\"\n"
	body, contentType := createMultipartFile(t, "csv_file", "readwise-data.csv", csvContent)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/readwise/import-csv", body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:readwise-csv-import-result")
}
