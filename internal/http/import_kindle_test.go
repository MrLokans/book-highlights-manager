package http

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockExporterForKindle struct {
	result exporters.ExportResult
	err    error
}

func (m *mockExporterForKindle) Export(_ []entities.Book) (exporters.ExportResult, error) {
	return m.result, m.err
}

func createMultipartFile(t *testing.T, fieldName, fileName, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	writer.Close()
	return body, writer.FormDataContentType()
}

func TestKindleImportController_ImportJSON_NoFile(t *testing.T) {
	exporter := &mockExporterForKindle{}
	controller := NewKindleImportController(exporter, nil)

	router := newTestRouter(t)
	router.POST("/import/kindle", controller.ImportJSON)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/kindle", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp KindleImportResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "not provided")
}

func TestKindleImportController_ImportJSON_ValidClippings(t *testing.T) {
	exporter := &mockExporterForKindle{result: exporters.ExportResult{BooksProcessed: 1, HighlightsProcessed: 2}}
	controller := NewKindleImportController(exporter, nil)

	router := newTestRouter(t)
	router.POST("/import/kindle", controller.ImportJSON)

	// Create a minimal valid Kindle clippings file
	clippings := `Test Book (Author Name)
- Your Highlight on page 1 | location 100-110 | Added on Monday, January 1, 2024 12:00:00 AM

This is a test highlight
==========
`
	body, contentType := createMultipartFile(t, "clippings_file", "My Clippings.txt", clippings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/import/kindle", body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp KindleImportResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestKindleImportController_Import_NoFile(t *testing.T) {
	exporter := &mockExporterForKindle{}
	controller := NewKindleImportController(exporter, nil)

	router := newTestRouter(t)
	router.POST("/settings/kindle/import", controller.Import)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/kindle/import", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:kindle-import-result")
}
