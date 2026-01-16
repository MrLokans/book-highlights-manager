package http

import (
	"html/template"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/analytics"
	"github.com/mrlokans/assistant/internal/auth"
)

const analyticsContextKey = "analytics_template_data"

// AnalyticsTemplateData holds Plausible analytics info for templates.
type AnalyticsTemplateData struct {
	Enabled   bool          // Whether analytics is enabled
	Domain    string        // Plausible domain
	ScriptTag template.HTML // Pre-generated script tag
}

// AnalyticsContextMiddleware injects analytics data into Gin context for templates.
// It stores both template data and the script URL for SecurityHeadersMiddleware to use.
// Must run BEFORE SecurityHeadersMiddleware in the middleware chain.
func AnalyticsContextMiddleware(store *analytics.PlausibleStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := store.GetEffectiveConfig()

		data := AnalyticsTemplateData{
			Enabled:   cfg.Enabled,
			Domain:    cfg.Domain,
			ScriptTag: analytics.GenerateScriptTag(cfg),
		}

		c.Set(analyticsContextKey, data)

		// Store script URL for SecurityHeadersMiddleware CSP
		if cfg.Enabled && cfg.ScriptURL != "" {
			c.Set(auth.AnalyticsScriptURLContextKey, cfg.ScriptURL)
		}

		c.Next()
	}
}

// GetAnalyticsTemplateData retrieves analytics data from context for use in templates.
func GetAnalyticsTemplateData(c *gin.Context) AnalyticsTemplateData {
	if data, exists := c.Get(analyticsContextKey); exists {
		if analyticsData, ok := data.(AnalyticsTemplateData); ok {
			return analyticsData
		}
	}
	return AnalyticsTemplateData{}
}
