package demo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewMiddleware(t *testing.T) {
	m := NewMiddleware(true)
	if !m.IsEnabled() {
		t.Error("Expected middleware to be enabled")
	}

	m = NewMiddleware(false)
	if m.IsEnabled() {
		t.Error("Expected middleware to be disabled")
	}
}

func TestMiddleware_AllowsGETRequests(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", w.Body.String())
	}
}

func TestMiddleware_BlocksPOSTRequests(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["demo_mode"] != true {
		t.Error("Expected demo_mode flag in response")
	}
}

func TestMiddleware_BlocksPUTRequests(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.PUT("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestMiddleware_BlocksDELETERequests(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.DELETE("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestMiddleware_AllowsHEADRequests(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.HEAD("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodHead, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMiddleware_AllowsOPTIONSRequests(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMiddleware_AllowsAuthEndpoints(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.POST("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	router.POST("/logout", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	router.POST("/auth/callback", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	testCases := []string{"/login", "/logout", "/auth/callback"}
	for _, path := range testCases {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for %s, got %d", path, w.Code)
		}
	}
}

func TestMiddleware_DisabledAllowsAllRequests(t *testing.T) {
	m := NewMiddleware(false)
	router := gin.New()
	router.Use(m.Handler())
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 when disabled, got %d", w.Code)
	}
}

func TestMiddleware_HTMXResponse(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.Handler())
	router.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}

	if w.Header().Get("HX-Reswap") != "none" {
		t.Error("Expected HX-Reswap header to be 'none'")
	}

	if w.Header().Get("HX-Trigger") == "" {
		t.Error("Expected HX-Trigger header to be set")
	}
}

func TestMiddleware_InjectContext(t *testing.T) {
	m := NewMiddleware(true)
	router := gin.New()
	router.Use(m.InjectContext())
	router.GET("/test", func(c *gin.Context) {
		val, exists := c.Get(ContextKeyDemoMode)
		if !exists {
			c.String(http.StatusInternalServerError, "context not set")
			return
		}
		if val.(bool) {
			c.String(http.StatusOK, "demo_enabled")
		} else {
			c.String(http.StatusOK, "demo_disabled")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Body.String() != "demo_enabled" {
		t.Errorf("Expected 'demo_enabled', got %s", w.Body.String())
	}
}

func TestMiddleware_InjectContextDisabled(t *testing.T) {
	m := NewMiddleware(false)
	router := gin.New()
	router.Use(m.InjectContext())
	router.GET("/test", func(c *gin.Context) {
		val, exists := c.Get(ContextKeyDemoMode)
		if !exists {
			c.String(http.StatusInternalServerError, "context not set")
			return
		}
		if val.(bool) {
			c.String(http.StatusOK, "demo_enabled")
		} else {
			c.String(http.StatusOK, "demo_disabled")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Body.String() != "demo_disabled" {
		t.Errorf("Expected 'demo_disabled', got %s", w.Body.String())
	}
}
