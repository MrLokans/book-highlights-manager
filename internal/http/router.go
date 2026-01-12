package http

import (
	"html/template"

	"github.com/gin-gonic/gin"
)

// NewRouter creates and configures the HTTP router with all endpoints.
// Uses RouterConfig to receive all dependencies, improving testability
// and reducing parameter count.
func NewRouter(cfg RouterConfig) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Load HTML templates
	tmpl := template.Must(template.ParseGlob(cfg.TemplatesPath + "/*.html"))
	router.SetHTMLTemplate(tmpl)

	// Serve static files
	router.Static("/static", cfg.StaticPath)

	// Create controllers with appropriate interfaces
	health := NewHealthController(cfg.Database, cfg.Version)
	readwiseImporter := NewReadwiseAPIImportController(cfg.BookExporter, cfg.ReadwiseToken, cfg.Auditor)
	moonReaderImporter := NewMoonReaderImportController(cfg.BookExporter, cfg.Auditor)
	readwiseCSVImporter := NewReadwiseCSVImportController(cfg.BookExporter)
	appleBooksImporter := NewAppleBooksImportController(cfg.BookExporter)
	booksController := NewBooksController(cfg.BookReader)
	uiController := NewUIController(cfg.BookReader)
	var metadataController *MetadataController
	if cfg.MetadataEnricher != nil {
		metadataController = NewMetadataController(cfg.MetadataEnricher, cfg.SyncProgress)
	}
	var coversController *CoversController
	if cfg.CoverCache != nil {
		coversController = NewCoversController(cfg.CoverCache, cfg.BookReader)
	}
	settingsController := NewSettingsController(
		cfg.DatabasePath,
		cfg.DropboxAppKey,
		cfg.MoonReaderDropboxPath,
		cfg.MoonReaderDatabasePath,
		cfg.MoonReaderOutputDir,
	)

	// Health endpoints
	router.GET("/health", health.Status)
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// Import endpoints
	router.POST("/import/moonreader", moonReaderImporter.Import)
	router.POST("/api/v2/highlights", readwiseImporter.Import)

	// Books API endpoints
	router.GET("/api/books", booksController.GetAllBooks)
	router.GET("/api/books/search", booksController.GetBookByTitleAndAuthor)
	router.GET("/api/books/stats", booksController.GetBookStats)

	// Book metadata enrichment endpoints
	if metadataController != nil {
		router.POST("/api/books/:id/enrich", metadataController.EnrichBook)
		router.PATCH("/api/books/:id/isbn", metadataController.UpdateISBN)
		router.POST("/api/books/enrich-all", metadataController.EnrichAllMissing)
		router.GET("/api/sync/metadata/status", metadataController.GetSyncStatus)
	}

	// Book cover endpoint
	if coversController != nil {
		router.GET("/api/books/:id/cover", coversController.GetCover)
	}

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
