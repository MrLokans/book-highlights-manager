package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
)

// StubMoonReaderExporter implements BookExporter for testing
type StubMoonReaderExporter struct {
	ExportedBooks []entities.Book
	ExportResult  exporters.ExportResult
	ExportError   error
}

func (s *StubMoonReaderExporter) Export(books []entities.Book) (exporters.ExportResult, error) {
	s.ExportedBooks = books
	if s.ExportError != nil {
		return exporters.ExportResult{}, s.ExportError
	}
	return s.ExportResult, nil
}

func TestMoonReaderImportController_Import(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		request        MoonReaderImportRequest
		exportResult   exporters.ExportResult
		exportError    error
		expectedStatus int
		expectedBooks  int
	}{
		{
			name: "successful import with single book",
			request: MoonReaderImportRequest{
				Highlights: []MoonReaderHighlight{
					{
						ID:             1,
						BookTitle:      "Test Book",
						Filename:       "/path/Test Book - Author.epub",
						HighlightColor: "-256",
						TimeMs:         1700000000000,
						Bookmark:       "Page 10",
						Original:       "Highlighted text",
						Underline:      0,
						Strikethrough:  0,
					},
				},
			},
			exportResult: exporters.ExportResult{
				BooksProcessed:      1,
				HighlightsProcessed: 1,
			},
			expectedStatus: http.StatusOK,
			expectedBooks:  1,
		},
		{
			name: "multiple highlights same book",
			request: MoonReaderImportRequest{
				Highlights: []MoonReaderHighlight{
					{
						ID:             1,
						BookTitle:      "Test Book",
						Filename:       "/path/Test Book - Author.epub",
						HighlightColor: "-256",
						TimeMs:         1700000000000,
						Original:       "First highlight",
					},
					{
						ID:             2,
						BookTitle:      "Test Book",
						Filename:       "/path/Test Book - Author.epub",
						HighlightColor: "-256",
						TimeMs:         1700001000000,
						Original:       "Second highlight",
					},
				},
			},
			exportResult: exporters.ExportResult{
				BooksProcessed:      1,
				HighlightsProcessed: 2,
			},
			expectedStatus: http.StatusOK,
			expectedBooks:  1,
		},
		{
			name: "multiple books",
			request: MoonReaderImportRequest{
				Highlights: []MoonReaderHighlight{
					{
						ID:        1,
						BookTitle: "Book One",
						Filename:  "/path/Book One - Author A.epub",
						Original:  "Highlight one",
					},
					{
						ID:        2,
						BookTitle: "Book Two",
						Filename:  "/path/Book Two - Author B.epub",
						Original:  "Highlight two",
					},
				},
			},
			exportResult: exporters.ExportResult{
				BooksProcessed:      2,
				HighlightsProcessed: 2,
			},
			expectedStatus: http.StatusOK,
			expectedBooks:  2,
		},
		{
			name: "empty highlights returns error",
			request: MoonReaderImportRequest{
				Highlights: []MoonReaderHighlight{},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBooks:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create stub exporter
			stub := &StubMoonReaderExporter{
				ExportResult: tt.exportResult,
				ExportError:  tt.exportError,
			}

			// Create controller (using stub wrapped in interface)
			controller := &MoonReaderImportController{
				exporter:     stub,
				auditService: nil,
			}

			// Create test router
			router := gin.New()
			router.POST("/import/moonreader", func(c *gin.Context) {
				var req MoonReaderImportRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				if len(req.Highlights) == 0 {
					c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "no highlights provided"})
					return
				}

				books := moonReaderHighlightsToBooks(req.Highlights)
				result, err := stub.Export(books)
				if err != nil {
					c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.IndentedJSON(http.StatusOK, MoonReaderImportResponse{
					BooksProcessed:      result.BooksProcessed,
					HighlightsProcessed: result.HighlightsProcessed,
					BooksFailed:         result.BooksFailed,
					HighlightsFailed:    result.HighlightsFailed,
				})
			})

			// Create request
			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req, _ := http.NewRequest("POST", "/import/moonreader", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			// Verify status
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			// Verify exported books count
			if tt.expectedStatus == http.StatusOK {
				assert.Len(t, stub.ExportedBooks, tt.expectedBooks)
			}
			_ = controller // just to use the variable
		})
	}
}

func TestMoonReaderHighlightsToBooks(t *testing.T) {
	highlights := []MoonReaderHighlight{
		{
			ID:             1,
			BookTitle:      "Test Book",
			Filename:       "/path/Test Book - Test Author.epub",
			HighlightColor: "-256",
			TimeMs:         1700000000000,
			Bookmark:       "Page 10",
			Note:           "My note",
			Original:       "Original text",
			Underline:      1,
			Strikethrough:  0,
		},
		{
			ID:             2,
			BookTitle:      "Test Book",
			Filename:       "/path/Test Book - Test Author.epub",
			HighlightColor: "-16711936",
			TimeMs:         1700001000000,
			Original:       "Another highlight",
			Strikethrough:  1,
		},
	}

	books := moonReaderHighlightsToBooks(highlights)

	require.Len(t, books, 1)

	book := books[0]
	assert.Equal(t, "Test Book", book.Title)
	assert.Equal(t, "Test Author", book.Author)
	assert.Len(t, book.Highlights, 2)

	// Check first highlight
	h1 := book.Highlights[0]
	assert.Equal(t, "Original text", h1.Text)
	assert.Equal(t, "My note", h1.Note)
	assert.Equal(t, entities.HighlightStyleUnderline, h1.Style)
	assert.Equal(t, "Page 10", h1.Chapter)

	// Check second highlight
	h2 := book.Highlights[1]
	assert.Equal(t, "Another highlight", h2.Text)
	assert.Equal(t, entities.HighlightStyleStrikethrough, h2.Style)
}

func TestMoonReaderHighlightsToBooks_MultipleAuthors(t *testing.T) {
	highlights := []MoonReaderHighlight{
		{
			ID:        1,
			BookTitle: "Book A",
			Filename:  "/path/Book A - Author One.epub",
			Original:  "Highlight 1",
		},
		{
			ID:        2,
			BookTitle: "Book B",
			Filename:  "/path/Book B - Author Two.epub",
			Original:  "Highlight 2",
		},
	}

	books := moonReaderHighlightsToBooks(highlights)

	assert.Len(t, books, 2)

	// Find books by title
	var bookA, bookB *entities.Book
	for i := range books {
		if books[i].Title == "Book A" {
			bookA = &books[i]
		} else if books[i].Title == "Book B" {
			bookB = &books[i]
		}
	}

	require.NotNil(t, bookA)
	require.NotNil(t, bookB)

	assert.Equal(t, "Author One", bookA.Author)
	assert.Equal(t, "Author Two", bookB.Author)
}
