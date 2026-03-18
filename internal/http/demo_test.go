package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/demo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoController_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns disabled when middleware is nil", func(t *testing.T) {
		controller := NewDemoController(nil)
		router := gin.New()
		router.GET("/api/demo/status", controller.GetStatus)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/demo/status", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp DemoStatusResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.False(t, resp.Enabled)
	})

	t.Run("returns enabled when demo mode active", func(t *testing.T) {
		middleware := demo.NewMiddleware(true)
		controller := NewDemoController(middleware)
		router := gin.New()
		router.GET("/api/demo/status", controller.GetStatus)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/demo/status", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp DemoStatusResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.True(t, resp.Enabled)
		assert.Contains(t, resp.Message, "write operations are blocked")
	})

	t.Run("returns disabled when middleware disabled", func(t *testing.T) {
		middleware := demo.NewMiddleware(false)
		controller := NewDemoController(middleware)
		router := gin.New()
		router.GET("/api/demo/status", controller.GetStatus)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/demo/status", nil)
		router.ServeHTTP(w, req)

		var resp DemoStatusResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.False(t, resp.Enabled)
	})
}
