package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVersionContextMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sets version in context", func(t *testing.T) {
		router := gin.New()
		router.Use(VersionContextMiddleware("1.2.3"))
		router.GET("/test", func(c *gin.Context) {
			version := GetVersionTemplateData(c)
			c.String(http.StatusOK, version)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "1.2.3", w.Body.String())
	})

	t.Run("returns dev when not set", func(t *testing.T) {
		router := gin.New()
		router.GET("/test", func(c *gin.Context) {
			version := GetVersionTemplateData(c)
			c.String(http.StatusOK, version)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "dev", w.Body.String())
	})
}

func TestGetDemoTemplateData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns false when not set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		data := GetDemoTemplateData(c)
		assert.False(t, data.Enabled)
	})
}
