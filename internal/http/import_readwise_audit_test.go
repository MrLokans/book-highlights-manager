package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadwiseHandlerWithAudit(t *testing.T) {
	// Create a temporary directory for audit files
	tempAuditDir := "./test_audit_integration"
	defer os.RemoveAll(tempAuditDir)

	// Create auditor
	auditor := audit.NewAuditor(tempAuditDir)

	// Set up router with auditor
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	stubExporter := new(StubExporter)
	readwiseImporter := NewReadwiseAPIImportController(stubExporter, TEST_READWISE_TOKEN, auditor)

	router.POST("/api/v2/highlights", readwiseImporter.Import)

	t.Run("Successful request creates audit file", func(t *testing.T) {
		requestModel := ReadwiseImportRequest{
			Highlights: []ReadwiseSingleHighlight{
				{
					Text:          "This is a test highlight",
					Title:         "Test Book",
					Author:        "Test Author",
					SourceType:    "book",
					Category:      "books",
					Page:          42,
					LocationType:  "page",
					HighlightedAt: "2023-01-01T00:00:00Z",
					Id:            "test-id-123",
				},
			},
		}
		body, _ := json.Marshal(requestModel)

		response := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(body))
		req.Header.Add("Authorization", "Token "+TEST_READWISE_TOKEN)
		router.ServeHTTP(response, req)

		assert.Equal(t, 200, response.Code)

		// Check that audit directory was created
		_, err := os.Stat(tempAuditDir)
		assert.NoError(t, err)

		// Check that an audit file was created
		files, err := os.ReadDir(tempAuditDir)
		require.NoError(t, err)
		assert.Len(t, files, 1)

		// Verify the audit file contains the correct data
		auditFile := files[0]
		assert.Contains(t, auditFile.Name(), ".json")

		auditFilePath := filepath.Join(tempAuditDir, auditFile.Name())
		auditContent, err := os.ReadFile(auditFilePath)
		require.NoError(t, err)

		var auditedRequest ReadwiseImportRequest
		err = json.Unmarshal(auditContent, &auditedRequest)
		require.NoError(t, err)

		assert.Len(t, auditedRequest.Highlights, 1)
		assert.Equal(t, "This is a test highlight", auditedRequest.Highlights[0].Text)
		assert.Equal(t, "Test Book", auditedRequest.Highlights[0].Title)
		assert.Equal(t, "Test Author", auditedRequest.Highlights[0].Author)
	})

	t.Run("Failed authentication does not create audit file", func(t *testing.T) {
		// Clear the audit directory
		os.RemoveAll(tempAuditDir)

		requestModel := ReadwiseImportRequest{
			Highlights: []ReadwiseSingleHighlight{
				{Text: "Should not be audited", Title: "Book", Author: "Author"},
			},
		}
		body, _ := json.Marshal(requestModel)

		response := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(body))
		// No authorization header
		router.ServeHTTP(response, req)

		assert.Equal(t, 401, response.Code)

		// Check that no audit directory was created (since request failed before audit)
		_, err := os.Stat(tempAuditDir)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("Invalid JSON does not create audit file", func(t *testing.T) {
		// Clear the audit directory
		os.RemoveAll(tempAuditDir)

		invalidJSON := []byte(`{"invalid": json}`)

		response := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v2/highlights", bytes.NewReader(invalidJSON))
		req.Header.Add("Authorization", "Token "+TEST_READWISE_TOKEN)
		router.ServeHTTP(response, req)

		assert.Equal(t, 400, response.Code)

		// Check that no audit directory was created (since JSON parsing failed before audit)
		_, err := os.Stat(tempAuditDir)
		assert.True(t, os.IsNotExist(err))
	})
}
