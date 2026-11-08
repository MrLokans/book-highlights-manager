package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNewRouter_HealthAndStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterConfig{
		TemplatesPath: "../../templates",
		StaticPath:    "../../static",
		Version:       "test",
	})

	serve := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		return w
	}

	t.Run("HEAD /health returns 200 for wget --spider", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, serve(http.MethodHead, "/health").Code)
	})

	t.Run("static files must be revalidated", func(t *testing.T) {
		w := serve(http.MethodGet, "/static/style.css")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	})

	t.Run("pages link version-tagged assets so CDN caches bust on release", func(t *testing.T) {
		body := serve(http.MethodGet, "/no-such-page").Body.String()
		assert.Contains(t, body, `href="/static/style.css?v=test"`)
		assert.Contains(t, body, `src="/static/app.js?v=test"`)
	})
}
