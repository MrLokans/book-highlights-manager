package http

import (
	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/demo"
)

// DemoTemplateData holds demo mode info for templates.
type DemoTemplateData struct {
	Enabled bool // Whether demo mode is active
}

// GetDemoTemplateData retrieves demo mode data from context for use in templates.
func GetDemoTemplateData(c *gin.Context) DemoTemplateData {
	enabled, _ := c.Get(demo.ContextKeyDemoMode)
	isEnabled, _ := enabled.(bool)

	return DemoTemplateData{
		Enabled: isEnabled,
	}
}
