package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/stretchr/testify/assert"
)

const TEST_READWISE_TOKEN = "test_token"

type StubExporter struct {
}

func (s *StubExporter) Export(books []entities.Book) (exporters.ExportResult, error) {
	processedBooks := 0
	processedHighlights := 0

	for _, book := range books {
		processedBooks++
		processedHighlights += len(book.Highlights)
	}

	return exporters.ExportResult{
		BooksProcessed:      processedBooks,
		HighlightsProcessed: processedHighlights,
	}, nil
}

func setupRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	stubExporter := new(StubExporter)
	readwiseImporter := NewReadwiseAPIImportController(stubExporter, TEST_READWISE_TOKEN, nil)

	router.POST("/api/v2/highlights", readwiseImporter.Import)
	return router
}

func getExportResultFromResponse(t testing.TB, body io.Reader) (result exporters.ExportResult) {
	t.Helper()
	err := json.NewDecoder(body).Decode(&result)

	if err != nil {
		t.Fatalf("Unable to parse response from server %q into struct", body)
	}
	return result
}

func TestReadwiseHandler(t *testing.T) {

	t.Run("Fails without the auth token", func(t *testing.T) {

		requestModel := ReadwiseImportRequest{
			Highlights: []ReadwiseSingleHighlight{},
		}
		body, _ := json.Marshal(requestModel)

		router := setupRouter()

		response := httptest.NewRecorder()
		// Prepend the auth token header
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(body))
		router.ServeHTTP(response, req)

		assert.Equal(t, 401, response.Code)
	})

	t.Run("Fails without the auth token", func(t *testing.T) {

		requestModel := ReadwiseImportRequest{
			Highlights: []ReadwiseSingleHighlight{},
		}
		body, _ := json.Marshal(requestModel)

		router := setupRouter()

		response := httptest.NewRecorder()
		// Prepend the auth token header
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(body))
		req.Header.Add("Authorization", "Token "+TEST_READWISE_TOKEN+"fail")
		router.ServeHTTP(response, req)

		assert.Equal(t, 401, response.Code)
	})

	t.Run("Accepts empty highlights list", func(t *testing.T) {

		requestModel := ReadwiseImportRequest{
			Highlights: []ReadwiseSingleHighlight{},
		}
		body, _ := json.Marshal(requestModel)

		router := setupRouter()

		response := httptest.NewRecorder()
		// Prepend the auth token header
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(body))
		req.Header.Add("Authorization", "Token "+TEST_READWISE_TOKEN)
		router.ServeHTTP(response, req)

		assert.Equal(t, 200, response.Code)

		got := getExportResultFromResponse(t, response.Body)

		assert.Equal(t, 0, got.BooksProcessed)
		assert.Equal(t, 0, got.HighlightsProcessed)
		assert.Equal(t, 0, got.BooksFailed)
		assert.Equal(t, 0, got.HighlightsFailed)
	})

	t.Run("Accepts multiple highlights for the same author", func(t *testing.T) {

		requestModel := ReadwiseImportRequest{
			Highlights: []ReadwiseSingleHighlight{
				{Text: "Highlight 1", Title: "Book 1", Author: "Author 1"},
				{Text: "Highlight 2", Title: "Book 1", Author: "Author 1"},
				{Text: "Highlight 3", Title: "Book 1", Author: "Author 1"},
			},
		}
		body, _ := json.Marshal(requestModel)

		router := setupRouter()

		response := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(body))
		req.Header.Add("Authorization", "Token "+TEST_READWISE_TOKEN)
		router.ServeHTTP(response, req)

		assert.Equal(t, 200, response.Code)

		got := getExportResultFromResponse(t, response.Body)

		assert.Equal(t, 1, got.BooksProcessed)
		assert.Equal(t, 3, got.HighlightsProcessed)
		assert.Equal(t, 0, got.BooksFailed)
		assert.Equal(t, 0, got.HighlightsFailed)
	})

	t.Run("Accepts multiple highlights for the multiple authors", func(t *testing.T) {

		requestModel := ReadwiseImportRequest{
			Highlights: []ReadwiseSingleHighlight{
				{Text: "Highlight 1", Title: "Book 1", Author: "Author 1"},
				{Text: "Highlight 2", Title: "Book 1", Author: "Author 1"},
				{Text: "Highlight 3", Title: "Book 1", Author: "Author 1"},
				{Text: "Highlight 4", Title: "Book 2", Author: "Author 1"},
				{Text: "Highlight 5", Title: "Book 2", Author: "Author 2"},
			},
		}
		body, _ := json.Marshal(requestModel)

		router := setupRouter()

		response := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(body))
		req.Header.Add("Authorization", "Token "+TEST_READWISE_TOKEN)
		router.ServeHTTP(response, req)

		assert.Equal(t, 200, response.Code)

		got := getExportResultFromResponse(t, response.Body)

		assert.Equal(t, 3, got.BooksProcessed)
		assert.Equal(t, 5, got.HighlightsProcessed)
		assert.Equal(t, 0, got.BooksFailed)
		assert.Equal(t, 0, got.HighlightsFailed)
	})

}
