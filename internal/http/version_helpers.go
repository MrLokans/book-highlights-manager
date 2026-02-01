package http

import "github.com/gin-gonic/gin"

const versionContextKey = "version_template_data"

// VersionContextMiddleware injects version data into Gin context for templates.
func VersionContextMiddleware(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(versionContextKey, version)
		c.Next()
	}
}

// GetVersionTemplateData retrieves version from context for use in templates.
func GetVersionTemplateData(c *gin.Context) string {
	if data, exists := c.Get(versionContextKey); exists {
		if version, ok := data.(string); ok {
			return version
		}
	}
	return "dev"
}
