package http

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/auth"
)

// DefaultUserID is used when running in single-user mode.
// Deprecated: Use GetUserID(c) instead to support multi-user mode.
const DefaultUserID = uint(0)

// GetUserID extracts the authenticated user's ID from the Gin context.
// Returns DefaultUserID (0) when auth is disabled or no user is authenticated.
func GetUserID(c *gin.Context) uint {
	return auth.GetUserID(c)
}

// --- Response Types ---

// ErrorResponse is the standard error response format for all API errors.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`    // machine-readable error code
	Details any    `json:"details,omitempty"` // additional context (validation errors, etc.)
}

// SuccessResponse is a standard success response with optional data.
type SuccessResponse struct {
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// PaginatedResponse wraps paginated data with metadata.
type PaginatedResponse struct {
	Data       any   `json:"data"`
	Total      int64 `json:"total"`
	Limit      int   `json:"limit"`
	Offset     int   `json:"offset"`
	HasMore    bool  `json:"has_more"`
	TotalPages int   `json:"total_pages,omitempty"`
}

// --- Error Response Helpers ---

// respondBadRequest sends a 400 Bad Request response.
func respondBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{Error: message})
}

// respondNotFound sends a 404 Not Found response.
func respondNotFound(c *gin.Context, resource string) {
	c.JSON(http.StatusNotFound, ErrorResponse{Error: resource + " not found"})
}

// respondInternalError logs the error and sends a 500 Internal Server Error response.
// The actual error is logged but not exposed to the client.
func respondInternalError(c *gin.Context, err error, context string) {
	log.Printf("Internal error (%s): %v", context, err)
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
}

// respondError sends an error response with the given status code.
// Use the specific helpers (respondBadRequest, respondNotFound, etc.) when possible.
func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{Error: message})
}

// --- Success Response Helpers ---

// respondSuccess sends a 200 OK response with a message.
func respondSuccess(c *gin.Context, message string) {
	c.JSON(http.StatusOK, SuccessResponse{Message: message})
}

// respondCreated sends a 201 Created response with data.
func respondCreated(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

// respondAccepted sends a 202 Accepted response (for async operations).
func respondAccepted(c *gin.Context, message string, data any) {
	c.JSON(http.StatusAccepted, SuccessResponse{Message: message, Data: data})
}

// --- Parameter Parsing ---

// parseIDParam extracts and validates an unsigned integer ID from URL parameters.
// Returns the parsed ID or responds with a 400 error and returns 0, false.
func parseIDParam(c *gin.Context, paramName string) (uint, bool) {
	idStr := c.Param(paramName)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondBadRequest(c, "invalid "+paramName)
		return 0, false
	}
	return uint(id), true
}

// parseQueryID extracts and validates an unsigned integer ID from query parameters.
// Returns the parsed ID or responds with a 400 error and returns 0, false.
func parseQueryID(c *gin.Context, paramName string) (uint, bool) {
	idStr := c.Query(paramName)
	if idStr == "" {
		respondBadRequest(c, paramName+" is required")
		return 0, false
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondBadRequest(c, "invalid "+paramName)
		return 0, false
	}
	return uint(id), true
}

// --- HTMX Support ---

// isHTMXRequest returns true if the request is an HTMX request.
func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

// respondHTMXOrJSON renders an HTML template for HTMX requests or returns JSON otherwise.
// For HTMX requests, it renders the template with the given data.
// For regular requests, it returns JSON with the given data.
func respondHTMXOrJSON(c *gin.Context, status int, template string, data any) {
	if isHTMXRequest(c) {
		c.HTML(status, template, data)
		return
	}
	c.JSON(status, data)
}

// --- Legacy Helpers (deprecated, use typed alternatives above) ---

// jsonError responds with a JSON error in a consistent format.
// Deprecated: Use respondBadRequest, respondNotFound, or respondError instead.
func jsonError(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{Error: message})
}

