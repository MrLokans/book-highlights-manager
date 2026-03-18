package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsController_SettingsPage(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", "", "", true, 4)

	router := newTestRouter(t)
	router.GET("/settings", controller.SettingsPage)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/settings", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:settings")
}

func TestSettingsController_InitDropboxAuth_NoAppKey(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", "", "", false, 0)

	router := newTestRouter(t)
	router.POST("/settings/oauth/dropbox/init", controller.InitDropboxAuth)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/oauth/dropbox/init", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:settings-error")
}

func TestSettingsController_CheckDropboxToken_NoTokenStore(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", "", "", false, 0)

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
	controller := NewSettingsController(nil, nil, "", "", "", "", false, 0)

	router := newTestRouter(t)
	router.POST("/settings/oauth/dropbox/disconnect", controller.DisconnectDropbox)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/settings/oauth/dropbox/disconnect", nil)
	router.ServeHTTP(w, req)

	// Without a token store, disconnect should still return a status
	assert.Contains(t, w.Body.String(), "TEMPLATE:dropbox-status")
}

// --- Pure utility function tests ---

func TestGenerateCodeVerifier(t *testing.T) {
	v, err := generateCodeVerifier()
	require.NoError(t, err)
	assert.NotEmpty(t, v)
	assert.GreaterOrEqual(t, len(v), 32)
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-verifier-12345"
	challenge := generateCodeChallenge(verifier)
	assert.NotEmpty(t, challenge)

	// Same input should produce same output
	challenge2 := generateCodeChallenge(verifier)
	assert.Equal(t, challenge, challenge2)

	// Different input should produce different output
	challenge3 := generateCodeChallenge("different-verifier")
	assert.NotEqual(t, challenge, challenge3)
}

func TestGenerateState(t *testing.T) {
	state, err := generateState()
	require.NoError(t, err)
	assert.NotEmpty(t, state)

	state2, err := generateState()
	require.NoError(t, err)
	assert.NotEqual(t, state, state2) // should be random
}

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

func TestCleanupOldPKCE(t *testing.T) {
	controller := NewSettingsController(nil, nil, "", "", "", "", false, 0)

	// Add some PKCE data
	controller.pkceStoreMu.Lock()
	controller.pkceStore["old-state"] = pkceData{
		codeVerifier: "old-verifier",
		createdAt:    time.Now().Add(-20 * time.Minute),
	}
	controller.pkceStore["new-state"] = pkceData{
		codeVerifier: "new-verifier",
		createdAt:    time.Now(),
	}
	controller.pkceStoreMu.Unlock()

	controller.cleanupOldPKCE()

	controller.pkceStoreMu.RLock()
	defer controller.pkceStoreMu.RUnlock()
	assert.NotContains(t, controller.pkceStore, "old-state")
	assert.Contains(t, controller.pkceStore, "new-state")
}
