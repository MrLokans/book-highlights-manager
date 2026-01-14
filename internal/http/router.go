package http

import (
	"html/template"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
)

// TagInfo holds tag ID and name for template rendering.
type TagInfo struct {
	ID   uint
	Name string
}

// collectBookTags gathers all unique tags from a book and its highlights.
func collectBookTags(book entities.Book) []TagInfo {
	tagMap := make(map[uint]TagInfo)

	// Collect book tags
	for _, tag := range book.Tags {
		tagMap[tag.ID] = TagInfo{ID: tag.ID, Name: tag.Name}
	}

	// Collect highlight tags
	for _, highlight := range book.Highlights {
		for _, tag := range highlight.Tags {
			tagMap[tag.ID] = TagInfo{ID: tag.ID, Name: tag.Name}
		}
	}

	// Convert to slice
	tags := make([]TagInfo, 0, len(tagMap))
	for _, tag := range tagMap {
		tags = append(tags, tag)
	}
	return tags
}

// NewRouter creates and configures the HTTP router with all endpoints.
// Uses RouterConfig to receive all dependencies, improving testability
// and reducing parameter count.
func NewRouter(cfg RouterConfig) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Define custom template functions
	funcMap := template.FuncMap{
		"collectBookTags": collectBookTags,
		"subtract": func(a, b int) int {
			return a - b
		},
	}

	// Load HTML templates with custom functions
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob(cfg.TemplatesPath + "/*.html"))
	router.SetHTMLTemplate(tmpl)

	// Serve static files
	router.Static("/static", cfg.StaticPath)

	// Create controllers with appropriate interfaces
	health := NewHealthController(cfg.Database, cfg.Version)
	readwiseImporter := NewReadwiseAPIImportController(cfg.BookExporter, cfg.ReadwiseToken, cfg.Auditor)
	moonReaderImporter := NewMoonReaderImportController(cfg.BookExporter, cfg.Auditor)
	readwiseCSVImporter := NewReadwiseCSVImportController(cfg.BookExporter)
	appleBooksImporter := NewAppleBooksImportController(cfg.BookExporter)
	kindleImporter := NewKindleImportController(cfg.BookExporter)
	booksController := NewBooksController(cfg.BookReader)
	uiController := NewUIController(cfg.BookReader, cfg.TagStore)
	var metadataController *MetadataController
	if cfg.MetadataEnricher != nil {
		metadataController = NewMetadataController(cfg.MetadataEnricher, cfg.SyncProgress, cfg.TaskClient)
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
		cfg.TaskClient != nil,
		cfg.TaskWorkers,
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

	// Tag management endpoints
	if cfg.TagStore != nil {
		tagsController := NewTagsController(cfg.TagStore, cfg.TaskClient)
		router.GET("/api/tags", tagsController.GetAllTags)
		router.POST("/api/tags", tagsController.CreateTag)
		router.DELETE("/api/tags/:id", tagsController.DeleteTag)
		router.GET("/api/tags/suggest", tagsController.TagSuggest)
		router.GET("/api/tags/:id/books", tagsController.GetBooksByTag)
		router.POST("/api/books/:id/tags", tagsController.AddTagToBook)
		router.DELETE("/api/books/:id/tags/:tagId", tagsController.RemoveTagFromBook)
		router.POST("/api/highlights/:id/tags", tagsController.AddTagToHighlight)
		router.DELETE("/api/highlights/:id/tags/:tagId", tagsController.RemoveTagFromHighlight)
		router.POST("/api/admin/tags/cleanup", tagsController.CleanupOrphanTags)
	}

	// Delete endpoints
	if cfg.DeleteStore != nil {
		deleteController := NewDeleteController(cfg.DeleteStore)
		router.DELETE("/api/books/:id", deleteController.DeleteBook)
		router.DELETE("/api/books/:id/permanent", deleteController.DeleteBookPermanently)
		router.DELETE("/api/highlights/:id", deleteController.DeleteHighlight)
		router.DELETE("/api/highlights/:id/permanent", deleteController.DeleteHighlightPermanently)
	}

	// Task management endpoints
	if cfg.TaskClient != nil {
		tasksController := NewTasksController(cfg.TaskClient)
		router.GET("/api/tasks/types", tasksController.ListTaskTypes)
		router.GET("/api/tasks/:id", tasksController.GetTaskStatus)
		router.POST("/api/tasks/:type/run", tasksController.RunTask)
	}

	// Favourites endpoints
	if cfg.FavouritesStore != nil {
		favouritesController := NewFavouritesController(cfg.FavouritesStore)
		router.POST("/api/highlights/:id/favourite", favouritesController.AddFavourite)
		router.DELETE("/api/highlights/:id/favourite", favouritesController.RemoveFavourite)
		router.GET("/api/highlights/favourites", favouritesController.ListFavourites)
		router.GET("/api/highlights/favourites/count", favouritesController.GetFavouriteCount)
		router.GET("/favourites", favouritesController.FavouritesPage)
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
	router.POST("/settings/kindle/import", kindleImporter.Import)
	router.POST("/import/kindle", kindleImporter.ImportJSON)

	// Export settings routes
	router.POST("/settings/export/markdown/save", settingsController.SaveExportPath)
	router.POST("/settings/export/markdown/reset", settingsController.ResetExportPath)

	return router
}
