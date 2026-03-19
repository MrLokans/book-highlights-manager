package http

import (
	"bytes"
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
