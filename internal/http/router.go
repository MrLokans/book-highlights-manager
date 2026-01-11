package http

import (
	"html/template"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/exporters"
)

func NewRouter(exporter *exporters.DatabaseMarkdownExporter, readwiseToken string, auditor *audit.Auditor, templatesPath string, staticPath string, databasePath string, dropboxAppKey string, moonReaderDropboxPath string, moonReaderDatabasePath string, moonReaderOutputDir string, db *database.Database, version string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Load HTML templates
	tmpl := template.Must(template.ParseGlob(templatesPath + "/*.html"))
	router.SetHTMLTemplate(tmpl)

	// Serve static files
	router.Static("/static", staticPath)

	health := NewHealthController(db, version)
	readwiseImporter := NewReadwiseAPIImportController(exporter, readwiseToken, auditor)
	moonReaderImporter := NewMoonReaderImportController(exporter, auditor)
	readwiseCSVImporter := NewReadwiseCSVImportController(exporter)
	appleBooksImporter := NewAppleBooksImportController(exporter)
	booksController := NewBooksController(exporter)
	uiController := NewUIController(exporter)
	settingsController := NewSettingsController(databasePath, dropboxAppKey, moonReaderDropboxPath, moonReaderDatabasePath, moonReaderOutputDir)

	router.GET("/health", health.Status)
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	router.POST("/import/moonreader", moonReaderImporter.Import)
	router.POST("/api/v2/highlights", readwiseImporter.Import)

	// Books API endpoints
	router.GET("/api/books", booksController.GetAllBooks)
	router.GET("/api/books/search", booksController.GetBookByTitleAndAuthor)
	router.GET("/api/books/stats", booksController.GetBookStats)

	// UI routes
	router.GET("/", uiController.BooksPage)
	router.GET("/ui/books/:id", uiController.BookPage)
	router.GET("/ui/books/:id/download", uiController.DownloadMarkdown)
	router.GET("/ui/books/search", uiController.SearchBooks)
	router.GET("/ui/download-all", uiController.DownloadAllMarkdown)

	// Settings routes
	router.GET("/settings", settingsController.SettingsPage)
	router.POST("/settings/oauth/dropbox/init", settingsController.InitDropboxAuth)
	router.GET("/settings/oauth/dropbox/callback", settingsController.DropboxCallback)
	router.POST("/settings/oauth/dropbox/check", settingsController.CheckDropboxToken)
	router.POST("/settings/oauth/dropbox/disconnect", settingsController.DisconnectDropbox)
	router.POST("/settings/moonreader/import", settingsController.ImportMoonReaderBackup)
	router.POST("/settings/readwise/import-csv", readwiseCSVImporter.Import)
	router.POST("/settings/applebooks/import", appleBooksImporter.Import)

	// Export settings routes
	router.POST("/settings/export/markdown/save", settingsController.SaveExportPath)
	router.POST("/settings/export/markdown/reset", settingsController.ResetExportPath)

	return router
}
