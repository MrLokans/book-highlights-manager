package http

import (
	"html/template"
	"testing"

	"github.com/gin-gonic/gin"
)

// allTemplateNames lists every template name referenced by c.HTML() or RenderPage()
// across all HTTP handlers. Stub templates are registered for each name so handlers
// can render without panicking.
//
// When adding a new template reference in a handler, add the name here too.
var allTemplateNames = []string{
	// RenderPage templates (full pages)
	"audit",
	"book",
	"books",
	"favourites",
	"profile",
	"settings",
	"vocabulary",

	// Partial/fragment templates (HTMX responses)
	"analytics-preview",
	"analytics-result",
	"analytics-settings",
	"applebooks-import-result",
	"book-list",
	"book-tags",
	"dropbox-status",
	"error",
	"favourite-button",
	"favourites-list",
	"highlight-tags",
	"import-result",
	"kindle-import-result",
	"obsidian-sync-result",
	"obsidian-sync-settings",
	"obsidian-sync-status",
	"password-result",
	"readwise-csv-import-result",
	"readwise-sync-result",
	"readwise-sync-settings",
	"readwise-sync-status",
	"settings-callback",
	"settings-error",
	"tags-cleanup-result",
	"tags-filter",
	"token-result",
	"vocabulary-list",
	"word-card",
	"word-detail",
}

// newTestRouter creates a Gin engine with stub templates registered for all handler template names.
// Handlers calling c.HTML() will render "TEMPLATE:<name>" instead of panicking.
func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	tmpl := template.New("")
	for _, name := range allTemplateNames {
		template.Must(tmpl.New(name).Parse("TEMPLATE:" + name))
	}
	router.SetHTMLTemplate(tmpl)
	return router
}
