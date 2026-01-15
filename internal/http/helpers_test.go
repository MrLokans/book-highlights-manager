package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestParseIDParam_Valid(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "123"}}

	id, ok := parseIDParam(c, "id")

	assert.True(t, ok)
	assert.Equal(t, uint(123), id)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestParseIDParam_Invalid(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	id, ok := parseIDParam(c, "id")

	assert.False(t, ok)
	assert.Equal(t, uint(0), id)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid id")
}

func TestParseIDParam_Negative(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "-1"}}

	id, ok := parseIDParam(c, "id")

	assert.False(t, ok)
	assert.Equal(t, uint(0), id)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseQueryID_Valid(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?book_id=456", nil)

	id, ok := parseQueryID(c, "book_id")

	assert.True(t, ok)
	assert.Equal(t, uint(456), id)
}

func TestParseQueryID_Missing(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	id, ok := parseQueryID(c, "book_id")

	assert.False(t, ok)
	assert.Equal(t, uint(0), id)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "book_id is required")
}

func TestIsHTMXRequest(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected bool
	}{
		{"HTMX request", "true", true},
		{"Non-HTMX request", "", false},
		{"Invalid header", "false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				c.Request.Header.Set("HX-Request", tt.header)
			}

			result := isHTMXRequest(c)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRespondHTMXOrJSON_JSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	respondHTMXOrJSON(c, http.StatusOK, "unused-template", gin.H{"message": "test"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"message":"test"`)
}

func TestJsonError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	jsonError(c, http.StatusNotFound, "resource not found")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"error":"resource not found"`)
}

func TestDefaultUserID(t *testing.T) {
	assert.Equal(t, uint(0), DefaultUserID)
}
