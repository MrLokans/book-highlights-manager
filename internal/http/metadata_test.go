package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataController_GetSyncStatus_NoProgress(t *testing.T) {
	// SyncProgress is nil — should return running=false
	controller := NewMetadataController(nil, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/sync/metadata/status", controller.GetSyncStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/sync/metadata/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SyncStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Running)
}

func TestMetadataController_GetSyncStatus_WithProgress(t *testing.T) {
	mockSync := &mockSyncProgressForHTTP{
		progress: &entities.SyncProgress{
			Status:     entities.SyncStatusRunning,
			TotalItems: 10,
			Processed:  5,
			Succeeded:  4,
			Failed:     1,
		},
	}

	syncProgress := database.NewMetadataSyncProgress(mockSync)
	controller := NewMetadataController(nil, syncProgress, nil)

	router := newTestRouter(t)
	router.GET("/api/sync/metadata/status", controller.GetSyncStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/sync/metadata/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SyncStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Running)
	assert.Equal(t, 10, resp.TotalItems)
	assert.Equal(t, 5, resp.Processed)
	assert.Equal(t, 50.0, resp.Progress)
}

func TestMetadataController_GetSyncStatus_HTMX(t *testing.T) {
	controller := NewMetadataController(nil, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/sync/metadata/status", controller.GetSyncStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/sync/metadata/status", nil)
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// HTMX response is raw HTML, not a template
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestMetadataController_EnrichBook_InvalidID(t *testing.T) {
	controller := NewMetadataController(nil, nil, nil)

	router := newTestRouter(t)
	router.POST("/api/books/:id/enrich", controller.EnrichBook)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/books/abc/enrich", nil)
	router.ServeHTTP(w, req)

	// Should return error for invalid ID
	assert.Contains(t, w.Body.String(), "invalid book ID")
}

// mockSyncProgressForHTTP implements database.SyncProgressDB
type mockSyncProgressForHTTP struct {
	progress *entities.SyncProgress
}

func (m *mockSyncProgressForHTTP) StartSync(_ int) error                         { return nil }
func (m *mockSyncProgressForHTTP) UpdateProgress(_, _, _, _ int, _ string) error { return nil }
func (m *mockSyncProgressForHTTP) CompleteSync(_ bool, _ string) error           { return nil }
func (m *mockSyncProgressForHTTP) IsSyncRunning() (bool, error)                  { return false, nil }
func (m *mockSyncProgressForHTTP) GetSyncProgress() (*entities.SyncProgress, error) {
	return m.progress, nil
}
