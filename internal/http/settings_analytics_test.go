package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mrlokans/assistant/internal/analytics"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
	settingsdb "github.com/mrlokans/assistant/internal/database/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAnalyticsTest(t *testing.T) (*AnalyticsSettingsController, func()) {
	t.Helper()
	dbPath := t.TempDir() + "/test_analytics.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	repo := settingsdb.NewRepository(db.DB)
	store := analytics.NewPlausibleStore(repo, config.Plausible{})
	controller := NewAnalyticsSettingsController(store)

	return controller, func() { db.Close() }
}

func TestAnalyticsSettingsController_GetSettings_JSON(t *testing.T) {
	controller, cleanup := setupAnalyticsTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/analytics", controller.GetAnalyticsSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/analytics", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp AnalyticsSettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestAnalyticsSettingsController_GetSettings_HTMX(t *testing.T) {
	controller, cleanup := setupAnalyticsTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/analytics", controller.GetAnalyticsSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/analytics", nil)
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:analytics-settings")
}

func TestAnalyticsSettingsController_SaveSettings(t *testing.T) {
	controller, cleanup := setupAnalyticsTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/analytics/save", controller.SaveAnalyticsSettings)

	form := url.Values{
		"domain":    {"example.com"},
		"script_url": {"https://plausible.io/js/script.js"},
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/analytics/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsSettingsController_ToggleAnalytics(t *testing.T) {
	controller, cleanup := setupAnalyticsTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/analytics/toggle", controller.ToggleAnalytics)

	form := url.Values{"enabled": {"true"}}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/analytics/toggle", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsSettingsController_ClearSettings(t *testing.T) {
	controller, cleanup := setupAnalyticsTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.POST("/settings/analytics/clear", controller.ClearAnalyticsSettings)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/analytics/clear", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAnalyticsSettingsController_PreviewScriptTag_JSON(t *testing.T) {
	controller, cleanup := setupAnalyticsTest(t)
	defer cleanup()

	router := newTestRouter(t)
	router.GET("/settings/analytics/preview", controller.PreviewScriptTag)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings/analytics/preview", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
