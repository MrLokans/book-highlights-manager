package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/demo"
)

// DemoController handles demo mode related endpoints.
type DemoController struct {
	middleware *demo.Middleware
}

// NewDemoController creates a new demo controller.
func NewDemoController(middleware *demo.Middleware) *DemoController {
	return &DemoController{
		middleware: middleware,
	}
}

// DemoStatusResponse contains demo mode status information.
type DemoStatusResponse struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

// GetStatus returns the current demo mode status.
// GET /api/demo/status
func (dc *DemoController) GetStatus(c *gin.Context) {
	if dc.middleware == nil || !dc.middleware.IsEnabled() {
		c.JSON(http.StatusOK, DemoStatusResponse{
			Enabled: false,
			Message: "Demo mode is not active",
		})
		return
	}

	c.JSON(http.StatusOK, DemoStatusResponse{
		Enabled: true,
		Message: "Demo mode is active - write operations are blocked",
	})
}
