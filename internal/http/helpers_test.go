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

	respondHTMXOrJSON(c, "unused-template", gin.H{"message": "test"})

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

func TestRespondNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondNotFound(c, "book")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "book not found")
}

func TestRespondInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondInternalError(c, assert.AnError, "test context")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondError(c, http.StatusConflict, "conflict message")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "conflict message")
}

func TestRespondAccepted(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	respondAccepted(c, "processing", gin.H{"id": 1})

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), "processing")
}

func TestGetUserID(t *testing.T) {
	t.Run("returns default when not set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)

		id := GetUserID(c)
		assert.Equal(t, DefaultUserID, id)
	})
}

func TestDefaultUserID(t *testing.T) {
	assert.Equal(t, uint(0), DefaultUserID)
}

// --- ParsePagination ---

func newPaginationContext(query string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test?"+query, nil)
	return c
}

func TestParsePagination_Defaults(t *testing.T) {
	c := newPaginationContext("")
	p := ParsePagination(c)
	assert.Equal(t, 50, p.Limit)
	assert.Equal(t, 0, p.Offset)
}

func TestParsePagination_CustomDefault(t *testing.T) {
	c := newPaginationContext("")
	p := ParsePagination(c, 25)
	assert.Equal(t, 25, p.Limit)
}

func TestParsePagination_QueryParams(t *testing.T) {
	c := newPaginationContext("limit=10&offset=20")
	p := ParsePagination(c)
	assert.Equal(t, 10, p.Limit)
	assert.Equal(t, 20, p.Offset)
}

func TestParsePagination_MaxLimit(t *testing.T) {
	c := newPaginationContext("limit=500")
	p := ParsePagination(c)
	assert.Equal(t, 100, p.Limit, "should be capped at 100")
}

func TestParsePagination_InvalidLimit(t *testing.T) {
	c := newPaginationContext("limit=abc")
	p := ParsePagination(c)
	assert.Equal(t, 50, p.Limit, "should fall back to default")
}

func TestParsePagination_NegativeLimit(t *testing.T) {
	c := newPaginationContext("limit=-5")
	p := ParsePagination(c)
	assert.Equal(t, 50, p.Limit, "should fall back to default")
}

func TestParsePagination_ZeroLimit(t *testing.T) {
	c := newPaginationContext("limit=0")
	p := ParsePagination(c)
	assert.Equal(t, 50, p.Limit, "zero should fall back to default")
}

func TestParsePagination_NegativeOffset(t *testing.T) {
	c := newPaginationContext("offset=-1")
	p := ParsePagination(c)
	assert.Equal(t, 0, p.Offset, "negative offset should fall back to 0")
}

func TestParsePagination_InvalidOffset(t *testing.T) {
	c := newPaginationContext("offset=xyz")
	p := ParsePagination(c)
	assert.Equal(t, 0, p.Offset, "should fall back to 0")
}

// --- NewPaginatedResponse ---

func TestNewPaginatedResponse(t *testing.T) {
	p := Pagination{Limit: 10, Offset: 0}
	resp := NewPaginatedResponse([]string{"a", "b"}, 25, p)
	assert.Equal(t, int64(25), resp.Total)
	assert.Equal(t, 10, resp.Limit)
	assert.Equal(t, 0, resp.Offset)
	assert.True(t, resp.HasMore)
	assert.Equal(t, 3, resp.TotalPages)
}

func TestNewPaginatedResponse_LastPage(t *testing.T) {
	p := Pagination{Limit: 10, Offset: 20}
	resp := NewPaginatedResponse([]string{"a"}, 25, p)
	// 20+10=30 < 25 → false, so no more pages
	assert.False(t, resp.HasMore)
}

func TestNewPaginatedResponse_ExactEnd(t *testing.T) {
	p := Pagination{Limit: 10, Offset: 20}
	resp := NewPaginatedResponse(nil, 30, p)
	assert.False(t, resp.HasMore) // 30 < 30 → false
}

func TestNewPaginatedResponse_Empty(t *testing.T) {
	p := Pagination{Limit: 10, Offset: 0}
	resp := NewPaginatedResponse(nil, 0, p)
	assert.False(t, resp.HasMore)
	assert.Equal(t, 0, resp.TotalPages)
}
