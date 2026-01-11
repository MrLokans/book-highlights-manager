package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HighlightDocument represents the structure from sample.json
type HighlightDocument struct {
	File    string `json:"file"`
	MD5Sum  string `json:"md5sum"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Entries []struct {
		Time    int64  `json:"time"`
		Page    int    `json:"page"`
		Sort    string `json:"sort"`
		Drawer  string `json:"drawer"`
		Text    string `json:"text"`
		Chapter string `json:"chapter"`
	} `json:"entries"`
}

type KOReaderSample struct {
	Version   string              `json:"version"`
	CreatedOn int64               `json:"created_on"`
	Documents []HighlightDocument `json:"documents"`
}

// convertKOReaderToReadwise converts KOReader format to Readwise format
func convertKOReaderToReadwise(koReader KOReaderSample) ReadwiseImportRequest {
	var highlights []ReadwiseSingleHighlight

	for _, doc := range koReader.Documents {
		for _, entry := range doc.Entries {
			highlight := ReadwiseSingleHighlight{
				Text:          entry.Text,
				Title:         doc.Title,
				Author:        doc.Author,
				SourceType:    "book",
				Category:      "books",
				Note:          "",
				Page:          entry.Page,
				LocationType:  "page",
				HighlightedAt: "2023-01-01T00:00:00Z", // Using a fixed date for testing
				Id:            "",                     // Will be generated
			}
			highlights = append(highlights, highlight)
		}
	}

	return ReadwiseImportRequest{
		Highlights: highlights,
	}
}

func TestReadwiseIntegration(t *testing.T) {
	// Create a temporary database
	dbPath := "./integration_test.db"
	defer os.Remove(dbPath)

	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create a temporary directory for markdown export
	tempDir := "./temp_export"
	defer os.RemoveAll(tempDir)
	err = os.MkdirAll(tempDir, 0755)
	require.NoError(t, err)

	// Create the combined exporter
	exporter := exporters.NewDatabaseMarkdownExporter(db, tempDir, "highlights")

	// Set up the router with the real exporter
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	readwiseImporter := NewReadwiseAPIImportController(exporter, "test_token", nil)
	router.POST("/api/v2/highlights", readwiseImporter.Import)

	t.Run("Upload sample.json via Readwise API", func(t *testing.T) {
		// Read and parse the sample.json file
		sampleData, err := os.ReadFile("../fixtures/highlights-sample.json")
		require.NoError(t, err)

		var koReaderSample KOReaderSample
		err = json.Unmarshal(sampleData, &koReaderSample)
		require.NoError(t, err)

		// Convert to Readwise format
		readwiseRequest := convertKOReaderToReadwise(koReaderSample)

		// Marshal the request
		requestBody, err := json.Marshal(readwiseRequest)
		require.NoError(t, err)

		// Create HTTP request
		req, err := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(requestBody))
		require.NoError(t, err)
		req.Header.Add("Authorization", "Token test_token")
		req.Header.Add("Content-Type", "application/json")

		// Execute request
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)

		// Verify response
		assert.Equal(t, 200, response.Code)

		var result ReadwiseImportResponse
		err = json.NewDecoder(response.Body).Decode(&result)
		require.NoError(t, err)

		// Verify the response shows correct processing
		assert.Equal(t, 5, result.BooksProcessed) // 5 books in sample.json
		assert.Greater(t, result.HighlightsProcessed, 0)
		assert.Equal(t, 0, result.BooksFailed)
		assert.Equal(t, 0, result.HighlightsFailed)

		t.Logf("Processed %d books and %d highlights", result.BooksProcessed, result.HighlightsProcessed)
	})

	t.Run("Verify data was stored in database", func(t *testing.T) {
		// Get all books from database
		books, err := db.GetAllBooks()
		require.NoError(t, err)

		// Verify we have the expected number of books
		assert.Len(t, books, 5)

		// Verify specific books exist
		expectedBooks := map[string]string{
			"The Software Architect Elevator: Redefining the Architect's Role in the Digital Enterprise":            "Gregor Hohpe",
			"Искусство войны. С комментариями, иллюстрациями и каллиграфией":                                        "Сунь-Цзы",
			"Путь джедая: Поиск собственной методики продуктивности":                                                "Максим Дорофеев",
			"The First 90 Days, Updated and Expanded: Proven Strategies for Getting Up to Speed Faster and Smarter": "Michael Watkins",
			"Software Architecture: The Hard Parts": "Neal Ford & Mark Richards & Pramod Sadalage & Zhamak Dehghani",
		}

		foundBooks := make(map[string]string)
		totalHighlights := 0

		for _, book := range books {
			foundBooks[book.Title] = book.Author
			totalHighlights += len(book.Highlights)

			// Verify each book has highlights
			assert.Greater(t, len(book.Highlights), 0, "Book %s should have highlights", book.Title)

			// Verify highlights have proper data
			for _, highlight := range book.Highlights {
				assert.NotEmpty(t, highlight.Text, "Highlight text should not be empty")
				assert.Greater(t, highlight.Page, 0, "Highlight page should be greater than 0")
				assert.NotZero(t, highlight.ID, "Highlight should have an ID")
				assert.Equal(t, book.ID, highlight.BookID, "Highlight should reference correct book")
			}
		}

		// Verify all expected books were found
		for expectedTitle, expectedAuthor := range expectedBooks {
			foundAuthor, exists := foundBooks[expectedTitle]
			assert.True(t, exists, "Book '%s' should exist in database", expectedTitle)
			if exists {
				assert.Equal(t, expectedAuthor, foundAuthor, "Author should match for book '%s'", expectedTitle)
			}
		}

		t.Logf("Found %d books with total of %d highlights", len(books), totalHighlights)
	})

	t.Run("Verify specific book details", func(t *testing.T) {
		// Test a specific book - The Software Architect Elevator
		book, err := db.GetBookByTitleAndAuthor(
			"The Software Architect Elevator: Redefining the Architect's Role in the Digital Enterprise",
			"Gregor Hohpe",
		)
		require.NoError(t, err)

		assert.Equal(t, "Gregor Hohpe", book.Author)
		assert.Greater(t, len(book.Highlights), 10) // Should have multiple highlights

		// Check for a specific highlight we know exists
		foundSpecificHighlight := false
		for _, highlight := range book.Highlights {
			if highlight.Page == 24 &&
				len(highlight.Text) > 100 &&
				highlight.Text[:20] == "Senior developer\nDev" {
				foundSpecificHighlight = true
				break
			}
		}
		assert.True(t, foundSpecificHighlight, "Should find the specific highlight from page 24")
	})

	t.Run("Verify markdown files were created", func(t *testing.T) {
		// Check that markdown files were created in the export directory
		exportDir := tempDir + "/highlights"

		// Check if export directory exists
		_, err := os.Stat(exportDir)
		assert.NoError(t, err, "Export directory should exist")

		// Read directory contents
		files, err := os.ReadDir(exportDir)
		require.NoError(t, err)

		// Should have markdown files for each book
		assert.Greater(t, len(files), 0, "Should have created markdown files")

		// Check for a specific markdown file in source subdirectory
		readwiseDir := exportDir + "/readwise"
		readwiseFiles, err := os.ReadDir(readwiseDir)
		require.NoError(t, err, "Should have created readwise source subdirectory")

		foundMarkdownFile := false
		expectedFilename := "The Software Architect Elevator: Redefining the Architect's Role in the Digital Enterprise.md"
		for _, file := range readwiseFiles {
			if file.Name() == expectedFilename {
				foundMarkdownFile = true

				// Read the file content to verify it contains highlights
				content, err := os.ReadFile(readwiseDir + "/" + file.Name())
				require.NoError(t, err)

				contentStr := string(content)
				assert.Contains(t, contentStr, "## Highlights:", "Markdown should contain highlights section")
				assert.Contains(t, contentStr, "Gregor Hohpe", "Markdown should contain author name")
				assert.Contains(t, contentStr, "Senior developer", "Markdown should contain specific highlight text")
				assert.Contains(t, contentStr, "content_source: readwise", "Markdown should contain source in frontmatter")
				break
			}
		}
		assert.True(t, foundMarkdownFile, "Should find the specific markdown file: %s", expectedFilename)
	})
}
