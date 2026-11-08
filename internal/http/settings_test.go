package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsController_SettingsPage(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", nil, nil, true, 4)

	router := newTestRouter(t)
	router.GET("/settings", controller.SettingsPage)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:settings")
}

func TestSettingsController_InitDropboxAuth_NoAppKey(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", nil, nil, false, 0)

	router := newTestRouter(t)
	router.POST("/settings/oauth/dropbox/init", controller.InitDropboxAuth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/oauth/dropbox/init", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:settings-error")
}

func TestSettingsController_CheckDropboxToken_NoTokenStore(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", nil, nil, false, 0)

	router := newTestRouter(t)
	router.POST("/settings/oauth/dropbox/check", controller.CheckDropboxToken)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/oauth/dropbox/check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should render dropbox-status template showing disconnected
	assert.Contains(t, w.Body.String(), "TEMPLATE:dropbox-status")
}

func TestSettingsController_DisconnectDropbox_NoTokenStore(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", nil, nil, false, 0)

	router := newTestRouter(t)
	router.POST("/settings/oauth/dropbox/disconnect", controller.DisconnectDropbox)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/oauth/dropbox/disconnect", nil)
	router.ServeHTTP(w, req)

	// Without a token store, disconnect should still return a status
	assert.Contains(t, w.Body.String(), "TEMPLATE:dropbox-status")
}

// --- Pure utility function tests ---

func TestGetEffectiveHost(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("uses X-Forwarded-Host when present", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("X-Forwarded-Host", "proxy.example.com")

		assert.Equal(t, "proxy.example.com", getEffectiveHost(c))
	})

	t.Run("handles comma-separated forwarded hosts", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("X-Forwarded-Host", "first.example.com, second.example.com")

		assert.Equal(t, "first.example.com", getEffectiveHost(c))
	})

	t.Run("falls back to request host", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://direct.example.com/", nil)

		assert.Equal(t, "direct.example.com", getEffectiveHost(c))
	})
}

func TestSettingsController_InitDropboxAuth_RedirectsAndStoresVerifier(t *testing.T) {
	controller := NewSettingsController(nil, nil, "app-key", "", nil, nil, false, 0)

	router := newTestRouter(t)
	router.POST("/settings/oauth/dropbox/init", controller.InitDropboxAuth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "http://app.example.com/settings/oauth/dropbox/init", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	assert.NoError(t, err)
	assert.Equal(t, "www.dropbox.com", loc.Host)
	assert.Equal(t, "app-key", loc.Query().Get("client_id"))
	assert.Equal(t, "http://app.example.com/settings/oauth/dropbox/callback", loc.Query().Get("redirect_uri"))

	cookie := flowCookie(t, w)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, dropboxCallback, cookie.Path)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", dropboxCallback, nil)
	c.Request.AddCookie(cookie)
	flow, ok := readDropboxFlow(c)
	require.True(t, ok)
	assert.Equal(t, loc.Query().Get("state"), flow.Get("state"))
	assert.NotEmpty(t, flow.Get("verifier"))
	assert.Equal(t, loc.Query().Get("redirect_uri"), flow.Get("redirect"))
}

func flowCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == dropboxFlowCookie {
			return c
		}
	}
	t.Fatal("no dropbox flow cookie set")
	return nil
}

func TestSettingsController_DropboxCallback_RejectsMissingOrMismatchedFlow(t *testing.T) {
	controller := NewSettingsController(nil, nil, "app-key", "", nil, nil, false, 0)
	router := newTestRouter(t)
	router.POST("/settings/oauth/dropbox/init", controller.InitDropboxAuth)
	router.GET(dropboxCallback, controller.DropboxCallback)

	init := httptest.NewRecorder()
	router.ServeHTTP(init, httptest.NewRequest("POST", "http://app.example.com/settings/oauth/dropbox/init", nil))
	cookie := flowCookie(t, init)

	t.Run("no flow cookie (other browser, or expired)", func(t *testing.T) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", dropboxCallback+"?code=c&state=s", nil))
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("state does not match the cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", dropboxCallback+"?code=c&state=forged", nil)
		req.AddCookie(cookie)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		// The pending flow is cleared even on failure (single use).
		assert.Equal(t, -1, flowCookie(t, w).MaxAge)
	})
}
