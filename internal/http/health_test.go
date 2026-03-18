package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping() error {
	return m.err
}

func TestHealthController_Status(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns healthy when database is connected", func(t *testing.T) {
		controller := NewHealthController(&mockPinger{}, "1.0.0")

		router := gin.New()
		router.GET("/health", controller.Status)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response.Status)
		assert.Equal(t, "1.0.0", response.Version)
		assert.Equal(t, "ok", response.Checks["database"])
		assert.NotEmpty(t, response.Time)
	})

	t.Run("returns healthy when pinger is nil", func(t *testing.T) {
		controller := NewHealthController(nil, "1.0.0")

		router := gin.New()
		router.GET("/health", controller.Status)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response.Status)
		assert.Equal(t, "not configured", response.Checks["database"])
	})

	t.Run("returns unhealthy when ping fails", func(t *testing.T) {
		controller := NewHealthController(&mockPinger{err: errors.New("connection refused")}, "1.0.0")

		router := gin.New()
		router.GET("/health", controller.Status)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var response HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "unhealthy", response.Status)
		assert.Contains(t, response.Checks["database"], "error")
	})

	t.Run("includes version in response", func(t *testing.T) {
		controller := NewHealthController(&mockPinger{}, "2.5.3")

		router := gin.New()
		router.GET("/health", controller.Status)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		var response HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "2.5.3", response.Version)
	})

	t.Run("includes timestamp in response", func(t *testing.T) {
		controller := NewHealthController(&mockPinger{}, "1.0.0")

		router := gin.New()
		router.GET("/health", controller.Status)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		var response HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.NotEmpty(t, response.Time)
		assert.Contains(t, response.Time, "T")
	})
}

func TestNewHealthController(t *testing.T) {
	t.Run("creates controller with pinger and version", func(t *testing.T) {
		pinger := &mockPinger{}
		controller := NewHealthController(pinger, "1.2.3")

		assert.NotNil(t, controller)
		assert.Equal(t, pinger, controller.pinger)
		assert.Equal(t, "1.2.3", controller.version)
	})

	t.Run("accepts nil pinger", func(t *testing.T) {
		controller := NewHealthController(nil, "1.0.0")

		assert.NotNil(t, controller)
		assert.Nil(t, controller.pinger)
	})

	t.Run("accepts empty version", func(t *testing.T) {
		controller := NewHealthController(&mockPinger{}, "")

		assert.Equal(t, "", controller.version)
	})
}

func TestHealthResponse(t *testing.T) {
	t.Run("JSON serialization works correctly", func(t *testing.T) {
		response := HealthResponse{
			Status:  "healthy",
			Time:    "2024-01-01T12:00:00Z",
			Version: "1.0.0",
			Checks: map[string]string{
				"database": "ok",
				"cache":    "ok",
			},
		}

		jsonBytes, err := json.Marshal(response)
		require.NoError(t, err)

		var deserialized HealthResponse
		err = json.Unmarshal(jsonBytes, &deserialized)
		require.NoError(t, err)

		assert.Equal(t, response.Status, deserialized.Status)
		assert.Equal(t, response.Time, deserialized.Time)
		assert.Equal(t, response.Version, deserialized.Version)
		assert.Equal(t, response.Checks["database"], deserialized.Checks["database"])
	})

	t.Run("omits empty version", func(t *testing.T) {
		response := HealthResponse{
			Status: "healthy",
			Time:   "2024-01-01T12:00:00Z",
			Checks: map[string]string{},
		}

		jsonBytes, err := json.Marshal(response)
		require.NoError(t, err)

		assert.NotContains(t, string(jsonBytes), "version")
	})
}
