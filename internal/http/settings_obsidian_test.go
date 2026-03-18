package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mrlokans/assistant/internal/database"
	settingsdb "github.com/mrlokans/assistant/internal/database/settings"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupObsidianTest(t *testing.T) (*ObsidianSyncController, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test_obsidian.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	repo := settingsdb.NewRepository(db.DB)
	store := settingsstore.New(repo)
	controller := NewObsidianSyncController(store, nil)

	return controller, func() { db.Close() }
}

func TestObsidianSyncController_GetSettings_JSON(t *testing.T) {
	controller, cleanup := setupObsidianTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/obsidian", controller.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/obsidian", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "config")
	assert.Contains(t, resp, "presets")
}

func TestObsidianSyncController_GetSettings_HTMX(t *testing.T) {
	controller, cleanup := setupObsidianTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/obsidian", controller.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/obsidian", nil)
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:obsidian-sync-settings")
}

func TestObsidianSyncController_UpdateSettings(t *testing.T) {
	controller, cleanup := setupObsidianTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/obsidian/save", controller.UpdateSettings)

	form := url.Values{
		"enabled":    {"true"},
		"export_dir": {t.TempDir()},
		"schedule":   {"0 * * * *"},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/obsidian/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestObsidianSyncController_UpdateSettings_InvalidSchedule(t *testing.T) {
	controller, cleanup := setupObsidianTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/obsidian/save", controller.UpdateSettings)

	form := url.Values{
		"schedule": {"not-a-cron"},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/obsidian/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestObsidianSyncController_ResetSettings(t *testing.T) {
	controller, cleanup := setupObsidianTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/obsidian/reset", controller.ResetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/obsidian/reset", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestObsidianSyncController_ValidateDirectory(t *testing.T) {
	controller, cleanup := setupObsidianTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/obsidian/validate-directory", controller.ValidateDirectory)

	t.Run("valid directory", func(t *testing.T) {
		form := url.Values{"path": {t.TempDir()}}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/settings/obsidian/validate-directory", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		form := url.Values{"path": {"/nonexistent/path/abc123"}}
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/settings/obsidian/validate-directory", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // Returns 200 with error in response body
	})
}

func TestObsidianSyncController_GetStatus(t *testing.T) {
	controller, cleanup := setupObsidianTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/obsidian/status", controller.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/obsidian/status", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestObsidianSyncController_NilStore(t *testing.T) {
	controller := NewObsidianSyncController(nil, nil)

	router := newTestRouter(t)
	router.GET("/settings/obsidian", controller.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/obsidian", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
