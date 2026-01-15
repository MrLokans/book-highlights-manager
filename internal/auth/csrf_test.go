package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCSRFMiddleware_SkipsBearerAuth(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	router := gin.New()
	// Pass nil for authService - skips token validation (legacy behavior)
	router.Use(CSRFMiddleware(secret, false, nil))
	router.POST("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Request with Bearer token should skip CSRF check (without validation when authService is nil)
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for Bearer auth request, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_AllowsGET(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	router := gin.New()
	router.Use(CSRFMiddleware(secret, false, nil))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// GET requests should be allowed without CSRF token
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 for GET request, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_BlocksPOSTWithoutToken(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	router := gin.New()
	router.Use(CSRFMiddleware(secret, false, nil))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// POST without CSRF token should be blocked
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 for POST without CSRF token, got %d", rr.Code)
	}
}

func TestCSRFMiddleware_SetsTokenInContext(t *testing.T) {
	secret := []byte("test-secret-key-32-bytes-long!!")

	var csrfToken string
	router := gin.New()
	router.Use(CSRFMiddleware(secret, false, nil))
	router.GET("/test", func(c *gin.Context) {
		csrfToken = GetCSRFToken(c)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if csrfToken == "" {
		t.Error("Expected CSRF token to be set in context")
	}
}

func TestGetCSRFToken_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	token := GetCSRFToken(c)
	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}
}

func TestGetCSRFToken_WithToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("csrf_token", "test-token-123")

	token := GetCSRFToken(c)
	if token != "test-token-123" {
		t.Errorf("Expected 'test-token-123', got '%s'", token)
	}
}

func TestCSRFTokenField_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	field := CSRFTokenField(c)
	if field != "" {
		t.Errorf("Expected empty field, got '%s'", field)
	}
}

func TestCSRFTokenField_WithToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("csrf_token", "abc123")

	field := CSRFTokenField(c)
	expected := `<input type="hidden" name="gorilla.csrf.Token" value="abc123">`
	if field != expected {
		t.Errorf("Expected '%s', got '%s'", expected, field)
	}
}

func TestIsAPIWithValidBearer_WithBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer token123")

	// With nil authService, should return true (no validation)
	if !isAPIWithValidBearer(c, nil) {
		t.Error("Expected isAPIWithValidBearer to return true")
	}
}

func TestIsAPIWithValidBearer_WithBasic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	if isAPIWithValidBearer(c, nil) {
		t.Error("Expected isAPIWithValidBearer to return false for Basic auth")
	}
}

func TestIsAPIWithValidBearer_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	if isAPIWithValidBearer(c, nil) {
		t.Error("Expected isAPIWithValidBearer to return false without auth header")
	}
}

func TestIsAPIWithValidBearer_CaseInsensitive(t *testing.T) {
	testCases := []string{
		"bearer token",
		"BEARER token",
		"Bearer token",
		"BeArEr token",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Request.Header.Set("Authorization", tc)

			// With nil authService, should return true (no validation)
			if !isAPIWithValidBearer(c, nil) {
				t.Errorf("Expected isAPIWithValidBearer to return true for '%s'", tc)
			}
		})
	}
}

func TestCSRFErrorHandler_JSON(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Accept", "application/json")

	csrfErrorHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestCSRFErrorHandler_HTML(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Accept", "text/html")

	csrfErrorHandler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}
