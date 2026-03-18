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

func setupReadwiseTest(t *testing.T) (*ReadwiseSyncController, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test_readwise.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	repo := settingsdb.NewRepository(db.DB)
	store := settingsstore.New(repo)
	controller := NewReadwiseSyncController(store, nil, nil)

	return controller, func() { db.Close() }
}

func TestReadwiseSyncController_GetSettings_JSON(t *testing.T) {
	controller, cleanup := setupReadwiseTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/readwise", controller.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/readwise", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "config")
}

func TestReadwiseSyncController_GetSettings_HTMX(t *testing.T) {
	controller, cleanup := setupReadwiseTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/readwise", controller.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/readwise", nil)
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:readwise-sync-settings")
}

func TestReadwiseSyncController_UpdateSettings(t *testing.T) {
	controller, cleanup := setupReadwiseTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/readwise/save", controller.UpdateSettings)

	form := url.Values{
		"enabled":  {"true"},
		"token":    {"test-token-12345678"},
		"schedule": {"0 */6 * * *"},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/readwise/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReadwiseSyncController_ResetSettings(t *testing.T) {
	controller, cleanup := setupReadwiseTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/readwise/reset", controller.ResetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/readwise/reset", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReadwiseSyncController_GetStatus_JSON(t *testing.T) {
	controller, cleanup := setupReadwiseTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/readwise/status", controller.GetStatus)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/readwise/status", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestReadwiseSyncController_NilStore(t *testing.T) {
	controller := NewReadwiseSyncController(nil, nil, nil)

	router := newTestRouter(t)
	router.GET("/settings/readwise", controller.GetSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/readwise", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
