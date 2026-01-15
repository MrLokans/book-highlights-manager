package importers

import (
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockExporter struct {
	exportedBooks []entities.Book
	returnError   error
}

func (m *mockExporter) Export(books []entities.Book) (services.ExportResult, error) {
	m.exportedBooks = books
	if m.returnError != nil {
		return services.ExportResult{}, m.returnError
	}

	highlightCount := 0
	for _, b := range books {
		highlightCount += len(b.Highlights)
	}

	return services.ExportResult{
		BooksProcessed:      len(books),
		HighlightsProcessed: highlightCount,
	}, nil
}

func TestPipeline_Import_GroupsByBook(t *testing.T) {
	exporter := &mockExporter{}
	pipeline := NewPipeline(exporter)

	converter := NewReadwiseConverter([]ReadwiseHighlight{
		{Title: "Book A", Author: "Author 1", Text: "Highlight 1"},
		{Title: "Book A", Author: "Author 1", Text: "Highlight 2"},
		{Title: "Book B", Author: "Author 2", Text: "Highlight 3"},
	})

	result, err := pipeline.Import(converter)

	require.NoError(t, err)
	assert.Equal(t, 2, result.BooksProcessed)
	assert.Equal(t, 3, result.HighlightsProcessed)
	assert.Len(t, exporter.exportedBooks, 2)

	// Verify highlights are grouped correctly
	bookHighlights := make(map[string]int)
	for _, b := range exporter.exportedBooks {
		bookHighlights[b.Title] = len(b.Highlights)
	}
	assert.Equal(t, 2, bookHighlights["Book A"])
	assert.Equal(t, 1, bookHighlights["Book B"])
}

func TestPipeline_Import_EmptyInput(t *testing.T) {
	exporter := &mockExporter{}
	pipeline := NewPipeline(exporter)

	converter := NewReadwiseConverter([]ReadwiseHighlight{})

	result, err := pipeline.Import(converter)

	require.NoError(t, err)
	assert.Equal(t, 0, result.BooksProcessed)
	assert.Nil(t, exporter.exportedBooks)
}

func TestPipeline_ImportBooks_Direct(t *testing.T) {
	exporter := &mockExporter{}
	pipeline := NewPipeline(exporter)

	books := []entities.Book{
		{
			Title:  "Test Book",
			Author: "Test Author",
			Highlights: []entities.Highlight{
				{Text: "Highlight 1"},
				{Text: "Highlight 2"},
			},
		},
	}

	result, err := pipeline.ImportBooks(books)

	require.NoError(t, err)
	assert.Equal(t, 1, result.BooksProcessed)
	assert.Equal(t, 2, result.HighlightsProcessed)
}

func TestReadwiseConverter(t *testing.T) {
	highlights := []ReadwiseHighlight{
		{
			Title:         "Test Book",
			Author:        "Test Author",
			Text:          "Some highlight",
			Note:          "My note",
			Page:          42,
			HighlightedAt: "2024-01-15",
			ID:            "abc123",
		},
	}

	converter := NewReadwiseConverter(highlights)
	result, source := converter.Convert()

	require.Len(t, result, 1)
	assert.Equal(t, "readwise", source.Name)
	assert.Equal(t, "Test Book", result[0].BookTitle)
	assert.Equal(t, "Test Author", result[0].BookAuthor)
	assert.Equal(t, "Some highlight", result[0].Text)
	assert.Equal(t, "My note", result[0].Note)
	assert.Equal(t, 42, result[0].Page)
	assert.Equal(t, "abc123", result[0].ExternalID)
}

func TestMoonReaderConverter(t *testing.T) {
	highlights := []MoonReaderHighlight{
		{
			ID:             123,
			BookTitle:      "Test Book",
			Filename:       "Author Name - Test Book.epub",
			Original:       "Highlighted text",
			Note:           "My note",
			HighlightColor: "-256",
			TimeMs:         1705320000000, // 2024-01-15
			Bookmark:       "Chapter 1",
			Underline:      1,
		},
	}

	converter := NewMoonReaderConverter(highlights)
	result, source := converter.Convert()

	require.Len(t, result, 1)
	assert.Equal(t, "moonreader", source.Name)
	assert.Equal(t, "Test Book", result[0].BookTitle)
	assert.Equal(t, "Highlighted text", result[0].Text)
	assert.Equal(t, "My note", result[0].Note)
	assert.Equal(t, "Chapter 1", result[0].Chapter)
	assert.Equal(t, entities.HighlightStyleUnderline, result[0].Style)
	assert.Equal(t, "123", result[0].ExternalID)
}

func TestReadwiseCSVConverter(t *testing.T) {
	rows := []ReadwiseCSVRow{
		{
			Highlight:     "Some highlight",
			BookTitle:     "Test Book",
			BookAuthor:    "Test Author",
			Note:          "My note",
			Color:         "yellow",
			LocationType:  "page",
			Location:      "42",
			HighlightedAt: "2024-01-15",
		},
	}

	converter := NewReadwiseCSVConverter(rows)
	result, source := converter.Convert()

	require.Len(t, result, 1)
	assert.Equal(t, "readwise", source.Name)
	assert.Equal(t, "Test Book", result[0].BookTitle)
	assert.Equal(t, "Test Author", result[0].BookAuthor)
	assert.Equal(t, "Some highlight", result[0].Text)
	assert.Equal(t, "#FFFF00", result[0].Color)
	assert.Equal(t, entities.LocationTypePage, result[0].LocationType)
	assert.Equal(t, 42, result[0].LocationValue)
}

func TestRawHighlight_GroupKey(t *testing.T) {
	h1 := RawHighlight{BookTitle: "Book", BookAuthor: "Author"}
	h2 := RawHighlight{BookTitle: "Book", BookAuthor: "Author"}
	h3 := RawHighlight{BookTitle: "Book", BookAuthor: "Different"}

	assert.Equal(t, h1.GroupKey(), h2.GroupKey())
	assert.NotEqual(t, h1.GroupKey(), h3.GroupKey())
}
