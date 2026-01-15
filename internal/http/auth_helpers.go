package http

import (
	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/config"
)

// AuthTemplateData holds authentication info for templates.
type AuthTemplateData struct {
	Enabled   bool   // Whether auth is enabled (AuthModeLocal)
	LoggedIn  bool   // Whether user is logged in
	Username  string // Current user's username (empty if not logged in)
	CSRFToken string // CSRF token for forms (empty when auth disabled)
}

// AuthContextMiddleware injects authentication data into Gin context for templates.
// Templates can access auth data via .Auth in the template data.
func AuthContextMiddleware(authMode config.AuthMode) gin.HandlerFunc {
	authEnabled := authMode == config.AuthModeLocal

	return func(c *gin.Context) {
		authData := AuthTemplateData{
			Enabled:   authEnabled,
			LoggedIn:  false,
			Username:  "",
			CSRFToken: auth.GetCSRFToken(c),
		}

		if authEnabled {
			userID := auth.GetUserID(c)
			if userID != 0 {
				authData.LoggedIn = true
				authData.Username = auth.GetUsername(c)
			}
		}

		c.Set("auth_template_data", authData)
		c.Next()
	}
}

// GetAuthTemplateData retrieves auth data from context for use in templates.
func GetAuthTemplateData(c *gin.Context) AuthTemplateData {
	if data, exists := c.Get("auth_template_data"); exists {
		if authData, ok := data.(AuthTemplateData); ok {
			return authData
		}
	}
	return AuthTemplateData{}
}
