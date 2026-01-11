package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database"
)

type HealthResponse struct {
	Status  string            `json:"status"`
	Time    string            `json:"time"`
	Version string            `json:"version,omitempty"`
	Checks  map[string]string `json:"checks"`
}

type HealthController struct {
	db      *database.Database
	version string
}

func NewHealthController(db *database.Database, version string) *HealthController {
	return &HealthController{
		db:      db,
		version: version,
	}
}

func (h *HealthController) Status(c *gin.Context) {
	checks := make(map[string]string)
	status := "healthy"

	// Check database connectivity
	if h.db != nil {
		sqlDB, err := h.db.DB.DB()
		if err != nil {
			checks["database"] = "error: " + err.Error()
			status = "unhealthy"
		} else if err := sqlDB.Ping(); err != nil {
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
