package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/analytics"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
)

// AnalyticsSettingsController handles Plausible analytics configuration
type AnalyticsSettingsController struct {
	store *analytics.PlausibleStore
}

func NewAnalyticsSettingsController(db *database.Database, envConfig config.Plausible) *AnalyticsSettingsController {
	return &AnalyticsSettingsController{
		store: analytics.NewPlausibleStore(db, envConfig),
	}
}

// GetPlausibleStore returns the underlying store for middleware use
func (c *AnalyticsSettingsController) GetPlausibleStore() *analytics.PlausibleStore {
	return c.store
}

// AnalyticsSettingsResponse is the response for analytics settings API
type AnalyticsSettingsResponse struct {
	Success          bool     `json:"success"`
	Error            string   `json:"error,omitempty"`
	Enabled          bool     `json:"enabled"`
	EnabledSource    string   `json:"enabled_source"`
	Domain           string   `json:"domain"`
	DomainSource     string   `json:"domain_source"`
	ScriptURL        string   `json:"script_url"`
	ScriptURLSource  string   `json:"script_url_source"`
	Extensions       []string `json:"extensions"`
	ExtensionsSource string   `json:"extensions_source"`
}

// GetAnalyticsSettings returns the current analytics settings
func (c *AnalyticsSettingsController) GetAnalyticsSettings(ctx *gin.Context) {
	info := c.store.GetSettingsInfo()

	resp := AnalyticsSettingsResponse{
		Success:          true,
		Enabled:          info.Enabled,
		EnabledSource:    info.EnabledSource,
		Domain:           info.Domain,
		DomainSource:     info.DomainSource,
		ScriptURL:        info.ScriptURL,
		ScriptURLSource:  info.ScriptURLSource,
		Extensions:       info.Extensions,
		ExtensionsSource: info.ExtensionsSource,
	}

	// Check Accept header for response format
	if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
		ctx.JSON(http.StatusOK, resp)
		return
	}

	// HTMX response
	ctx.HTML(http.StatusOK, "analytics-settings", gin.H{
		"Settings":        resp,
		"ValidExtensions": analytics.ValidExtensions,
		"Auth":            GetAuthTemplateData(ctx),
		"Demo":            GetDemoTemplateData(ctx),
	})
}

// AnalyticsSaveRequest is the request for saving analytics settings
type AnalyticsSaveRequest struct {
	Enabled    *bool  `form:"enabled" json:"enabled"`
	Domain     string `form:"domain" json:"domain"`
	ScriptURL  string `form:"script_url" json:"script_url"`
	Extensions string `form:"extensions" json:"extensions"` // comma-separated
}

// SaveAnalyticsSettings saves the analytics configuration to the database
func (c *AnalyticsSettingsController) SaveAnalyticsSettings(ctx *gin.Context) {
	var req AnalyticsSaveRequest
	if err := ctx.ShouldBind(&req); err != nil {
		c.respondError(ctx, "Invalid request: "+err.Error())
		return
	}

	// Save enabled state if provided
	if req.Enabled != nil {
		if err := c.store.SetEnabled(*req.Enabled); err != nil {
			c.respondError(ctx, "Failed to save enabled state: "+err.Error())
			return
		}
	}

	// Save domain if provided
	domain := strings.TrimSpace(req.Domain)
	if domain != "" {
		if err := c.store.SetDomain(domain); err != nil {
			c.respondError(ctx, "Failed to save domain: "+err.Error())
			return
		}
	}

	// Save script URL if provided
	scriptURL := strings.TrimSpace(req.ScriptURL)
	if scriptURL != "" {
		if err := c.store.SetScriptURL(scriptURL); err != nil {
			c.respondError(ctx, "Failed to save script URL: "+err.Error())
			return
		}
	}

	// Save extensions if provided
	if req.Extensions != "" {
		extensions := parseExtensionsList(req.Extensions)
		// Validate extensions
		for _, ext := range extensions {
			if !analytics.IsValidExtension(ext) {
				c.respondError(ctx, "Invalid extension: "+ext)
				return
			}
		}
		if err := c.store.SetExtensions(extensions); err != nil {
			c.respondError(ctx, "Failed to save extensions: "+err.Error())
			return
		}
	}

	// Return updated settings
	c.respondSuccess(ctx)
}

// ClearAnalyticsSettings removes all database settings, reverting to environment/defaults
func (c *AnalyticsSettingsController) ClearAnalyticsSettings(ctx *gin.Context) {
	if err := c.store.ClearSettings(); err != nil {
		c.respondError(ctx, "Failed to clear settings: "+err.Error())
		return
	}

	c.respondSuccess(ctx)
}

// ToggleAnalytics enables or disables analytics
func (c *AnalyticsSettingsController) ToggleAnalytics(ctx *gin.Context) {
	var req struct {
		Enabled bool `form:"enabled" json:"enabled"`
	}
	if err := ctx.ShouldBind(&req); err != nil {
		c.respondError(ctx, "Invalid request: "+err.Error())
		return
	}

	if err := c.store.SetEnabled(req.Enabled); err != nil {
		c.respondError(ctx, "Failed to toggle analytics: "+err.Error())
		return
	}

	c.respondSuccess(ctx)
}

// PreviewScriptTag returns the generated script tag for preview
func (c *AnalyticsSettingsController) PreviewScriptTag(ctx *gin.Context) {
	cfg := c.store.GetEffectiveConfig()
	scriptTag := analytics.GenerateScriptTag(cfg)

	if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
		ctx.JSON(http.StatusOK, gin.H{
			"script_tag": string(scriptTag),
			"enabled":    cfg.Enabled,
			"domain":     cfg.Domain,
		})
		return
	}

	ctx.HTML(http.StatusOK, "analytics-preview", gin.H{
		"ScriptTag": scriptTag,
		"Enabled":   cfg.Enabled,
		"Domain":    cfg.Domain,
	})
}

func (c *AnalyticsSettingsController) respondError(ctx *gin.Context, errMsg string) {
	if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   errMsg,
		})
		return
	}

	ctx.HTML(http.StatusBadRequest, "analytics-result", gin.H{
		"Success": false,
		"Error":   errMsg,
	})
}

func (c *AnalyticsSettingsController) respondSuccess(ctx *gin.Context) {
	info := c.store.GetSettingsInfo()

	if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
		ctx.JSON(http.StatusOK, AnalyticsSettingsResponse{
			Success:          true,
			Enabled:          info.Enabled,
			EnabledSource:    info.EnabledSource,
			Domain:           info.Domain,
			DomainSource:     info.DomainSource,
			ScriptURL:        info.ScriptURL,
			ScriptURLSource:  info.ScriptURLSource,
			Extensions:       info.Extensions,
			ExtensionsSource: info.ExtensionsSource,
		})
		return
	}

	ctx.HTML(http.StatusOK, "analytics-result", gin.H{
		"Success":         true,
		"Settings":        info,
		"ValidExtensions": analytics.ValidExtensions,
	})
}

func parseExtensionsList(s string) []string {
	if s == "" {
		return []string{}
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
