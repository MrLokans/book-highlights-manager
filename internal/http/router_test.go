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

func TestNewRouter_ClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ipOf := func(header string, set map[string]string) string {
		router := NewRouter(RouterConfig{TemplatesPath: "../../templates", StaticPath: "../../static", ClientIPHeader: header})
		var got string
		router.GET("/ip-probe", func(c *gin.Context) { got = c.ClientIP() })
		req := httptest.NewRequest(http.MethodGet, "/ip-probe", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		for k, v := range set {
			req.Header.Set(k, v)
		}
		router.ServeHTTP(httptest.NewRecorder(), req)
		return got
	}

	assert.Equal(t, "10.0.0.1", ipOf("", map[string]string{"X-Forwarded-For": "6.6.6.6"}), "spoofed XFF must be ignored")
	assert.Equal(t, "1.2.3.4", ipOf("CF-Connecting-IP", map[string]string{"CF-Connecting-IP": "1.2.3.4", "X-Forwarded-For": "6.6.6.6"}))
}
