package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DatabasePinger checks database connectivity.
type DatabasePinger interface {
	Ping() error
}

// HealthResponse is the JSON payload for the health check endpoint.
type HealthResponse struct {
	Status  string            `json:"status"`
	Time    string            `json:"time"`
	Version string            `json:"version,omitempty"`
	Checks  map[string]string `json:"checks"`
}

// HealthController handles health check endpoints.
type HealthController struct {
	pinger  DatabasePinger
	version string
}

// NewHealthController creates a health controller.
func NewHealthController(pinger DatabasePinger, version string) *HealthController {
	return &HealthController{
		pinger:  pinger,
		version: version,
	}
}

// Status returns the health check response.
func (h *HealthController) Status(c *gin.Context) {
	checks := make(map[string]string)
	status := "healthy"

	// Check database connectivity
	if h.pinger != nil {
		if err := h.pinger.Ping(); err != nil {
			checks["database"] = "error: " + err.Error()
			status = "unhealthy"
		} else {
			checks["database"] = "ok"
		}
	} else {
		checks["database"] = "not configured"
	}

	health := HealthResponse{
		Status:  status,
		Time:    time.Now().Format(time.RFC3339),
		Version: h.version,
		Checks:  checks,
	}

	statusCode := http.StatusOK
	if status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.IndentedJSON(statusCode, health)
}
