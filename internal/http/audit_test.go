package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/database"
	auditdb "github.com/mrlokans/assistant/internal/database/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuditTest(t *testing.T) (*AuditController, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test_audit.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	repo := auditdb.NewRepository(db.DB)
	service := audit.NewService(repo)
	controller := NewAuditController(service)

	return controller, func() { db.Close() }
}

func TestAuditController_AuditLogPage(t *testing.T) {
	controller, cleanup := setupAuditTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/audit", controller.AuditLogPage)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/audit", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:audit")
}

func TestAuditController_GetAuditEvents_JSON(t *testing.T) {
	controller, cleanup := setupAuditTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/api/audit", controller.GetAuditEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "events")
	assert.Contains(t, resp, "total_events")
	assert.Equal(t, float64(1), resp["page"])
}

func TestAuditController_GetAuditEvents_WithPagination(t *testing.T) {
	controller, cleanup := setupAuditTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/api/audit", controller.GetAuditEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit?page=2&limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(2), resp["page"])
	assert.Equal(t, float64(10), resp["limit"])
}

func TestAuditController_GetAuditEvents_WithTypeFilter(t *testing.T) {
	controller, cleanup := setupAuditTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/api/audit", controller.GetAuditEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit?type=import", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetEventTypes(t *testing.T) {
	types := getEventTypes()
	assert.GreaterOrEqual(t, len(types), 7)
	assert.Equal(t, "", types[0].Value) // "All Events"
}
