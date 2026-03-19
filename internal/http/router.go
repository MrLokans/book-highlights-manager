package http

import (
	"html/template"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/logging"
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
func applyMiddleware(router *gin.Engine, cfg RouterConfig) {
	router.Use(logging.RequestIDMiddleware())

	if cfg.Core.PlausibleStore != nil {
		router.Use(AnalyticsContextMiddleware(cfg.Core.PlausibleStore))
	}
	router.Use(auth.SecurityHeadersMiddleware())

	if len(cfg.Auth.CSRFSecret) > 0 {
		router.Use(auth.CSRFMiddleware(cfg.Auth.CSRFSecret, cfg.Auth.SecureCookies, cfg.Auth.Service))
	}
	if cfg.Auth.SessionManager != nil {
		router.Use(cfg.Auth.SessionManager.SessionLoadSave())
	}

	if cfg.Auth.Middleware != nil {
		router.Use(cfg.Auth.Middleware.Handler())
	} else {
		router.Use(func(c *gin.Context) {
			c.Set(auth.ContextKeyUserID, auth.DefaultUserID)
			c.Set(auth.ContextKeyType, auth.TypeNone)
			c.Next()
		})
	}

	router.Use(AuthContextMiddleware(cfg.Auth.Config.Mode))
	router.Use(VersionContextMiddleware(cfg.Core.Version))

	if cfg.Core.DemoMiddleware != nil && cfg.Core.DemoMiddleware.IsEnabled() {
		router.Use(cfg.Core.DemoMiddleware.InjectContext())
		router.Use(cfg.Core.DemoMiddleware.Handler())
	}
}

func loadTemplates(router *gin.Engine, cfg RouterConfig) {
	funcMap := template.FuncMap{
		"collectBookTags": collectBookTags,
		"subtract":        func(a, b int) int { return a - b },
		"add":             func(a, b int) int { return a + b },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob(cfg.UI.TemplatesPath + "/*.html"))
	router.SetHTMLTemplate(tmpl)
}

// NewRouter creates the Gin router with all middleware, templates, and route registrations.
func NewRouter(cfg RouterConfig) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	applyMiddleware(router, cfg)
	loadTemplates(router, cfg)

	// Serve static files
	router.Static("/static", cfg.UI.StaticPath)

	// Register auth routes if auth service is available
	if cfg.Auth.Service != nil && cfg.Auth.Service.IsAuthEnabled() {
		authController, err := auth.NewController(cfg.Auth.Service, cfg.Auth.SessionManager, cfg.UI.TemplatesPath, cfg.Auth.Config)
		if err == nil {
			authController.RegisterRoutes(router)

			// API token management endpoints
			tokenController := auth.NewAPITokenController(cfg.Auth.Service)
			router.POST("/api/auth/token", tokenController.GenerateToken)
			router.DELETE("/api/auth/token", tokenController.RevokeToken)

			// Profile routes
			profileController := NewProfileController(cfg.Auth.Service)
			router.GET("/profile", profileController.ProfilePage)
			router.POST("/profile/password", profileController.ChangePassword)
			router.POST("/profile/token", profileController.GenerateToken)
			router.POST("/profile/token/regenerate", profileController.RegenerateToken)
			router.DELETE("/profile/token", profileController.RevokeToken)
		}
	}

	// Create controllers with appropriate interfaces
	health := NewHealthController(cfg.Core.Pinger, cfg.Core.Version)
	readwiseImporter := NewReadwiseAPIImportController(cfg.Core.BookExporter, cfg.Import.ReadwiseToken, cfg.Core.AuditService)
	moonReaderImporter := NewMoonReaderImportController(cfg.Core.BookExporter, cfg.Core.AuditService)
	readwiseCSVImporter := NewReadwiseCSVImportController(cfg.Core.BookExporter, cfg.Core.AuditService)
	appleBooksImporter := NewAppleBooksImportController(cfg.Core.BookExporter, cfg.Core.AuditService)
	kindleImporter := NewKindleImportController(cfg.Core.BookExporter, cfg.Core.AuditService)
	booksController := NewBooksController(cfg.Core.BookReader)
	uiController := NewUIController(cfg.Core.BookReader, cfg.Stores.TagStore, cfg.Stores.VocabularyStore)
	var metadataController *MetadataController
	if cfg.Metadata.Enricher != nil {
		metadataController = NewMetadataController(cfg.Metadata.Enricher, cfg.Metadata.SyncProgress, cfg.Tasks.Client)
	}
	var coversController *CoversController
	if cfg.Metadata.CoverCache != nil {
		coversController = NewCoversController(cfg.Metadata.CoverCache, cfg.Core.BookReader)
	}
	settingsController := NewSettingsController(
		cfg.Core.SettingsStore,
		cfg.Import.TokenStore,
		cfg.Import.DropboxAppKey,
		cfg.Import.MoonReaderDropboxPath,
		cfg.Import.MoonReaderDatabasePath,
		cfg.Import.MoonReaderOutputDir,
		cfg.Tasks.Client != nil,
		cfg.Tasks.Workers,
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
	if cfg.Stores.TagStore != nil {
		tagsController := NewTagsController(cfg.Stores.TagStore, cfg.Tasks.Client)
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
	if cfg.Stores.DeleteStore != nil {
		deleteController := NewDeleteController(cfg.Stores.DeleteStore, cfg.Core.AuditService)
		router.DELETE("/api/books/:id", deleteController.DeleteBook)
		router.DELETE("/api/books/:id/permanent", deleteController.DeleteBookPermanently)
		router.DELETE("/api/highlights/:id", deleteController.DeleteHighlight)
		router.DELETE("/api/highlights/:id/permanent", deleteController.DeleteHighlightPermanently)
	}

	// Task management endpoints
	if cfg.Tasks.Client != nil {
		tasksController := NewTasksController(cfg.Tasks.Client)
		router.GET("/api/tasks/types", tasksController.ListTaskTypes)
		router.GET("/api/tasks/:id", tasksController.GetTaskStatus)
		router.POST("/api/tasks/:type/run", tasksController.RunTask)
	}

	// Favourites endpoints
	if cfg.Stores.FavouritesStore != nil {
		favouritesController := NewFavouritesController(cfg.Stores.FavouritesStore)
		router.POST("/api/highlights/:id/favourite", favouritesController.AddFavourite)
		router.DELETE("/api/highlights/:id/favourite", favouritesController.RemoveFavourite)
		router.GET("/api/highlights/favourites", favouritesController.ListFavourites)
		router.GET("/api/highlights/favourites/count", favouritesController.GetFavouriteCount)
		router.GET("/favourites", favouritesController.FavouritesPage)
	}

	// Vocabulary endpoints
	if cfg.Stores.VocabularyStore != nil {
		vocabController := NewVocabularyController(cfg.Stores.VocabularyStore, cfg.Core.DictionaryClient, cfg.Tasks.Client)
		router.GET("/api/vocabulary", vocabController.ListWords)
		router.GET("/api/vocabulary/words", vocabController.GetWordsList)
		router.POST("/api/vocabulary", vocabController.AddWord)
		router.GET("/api/vocabulary/stats", vocabController.GetVocabularyStats)
		router.GET("/api/vocabulary/search", vocabController.SearchWords)
		router.GET("/api/vocabulary/:id", vocabController.GetWord)
		router.PATCH("/api/vocabulary/:id", vocabController.UpdateWord)
		router.DELETE("/api/vocabulary/:id", vocabController.DeleteWord)
		router.POST("/api/vocabulary/:id/enrich", vocabController.EnrichWord)
		router.POST("/api/vocabulary/enrich-all", vocabController.EnrichAllWords)
		router.GET("/api/highlights/:id/vocabulary", vocabController.GetWordsByHighlight)
		router.GET("/vocabulary", vocabController.VocabularyPage)
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

	// Demo mode status endpoint (always available)
	demoController := NewDemoController(cfg.Core.DemoMiddleware)
	router.GET("/api/demo/status", demoController.GetStatus)

	// Analytics settings routes (if PlausibleStore is available)
	if cfg.Core.PlausibleStore != nil {
		analyticsController := NewAnalyticsSettingsController(cfg.Core.PlausibleStore)
		router.GET("/settings/analytics", analyticsController.GetAnalyticsSettings)
		router.POST("/settings/analytics/save", analyticsController.SaveAnalyticsSettings)
		router.POST("/settings/analytics/clear", analyticsController.ClearAnalyticsSettings)
		router.POST("/settings/analytics/toggle", analyticsController.ToggleAnalytics)
		router.GET("/settings/analytics/preview", analyticsController.PreviewScriptTag)
	}

	// Obsidian sync settings routes (if SettingsStore is available)
	if cfg.Core.SettingsStore != nil {
		obsidianSyncController := NewObsidianSyncController(cfg.Core.SettingsStore, cfg.Sync.ObsidianScheduler)
		router.GET("/settings/obsidian", obsidianSyncController.GetSettings)
		router.POST("/settings/obsidian/save", obsidianSyncController.UpdateSettings)
		router.POST("/settings/obsidian/reset", obsidianSyncController.ResetSettings)
		router.POST("/settings/obsidian/validate-directory", obsidianSyncController.ValidateDirectory)
		router.POST("/settings/obsidian/sync-now", obsidianSyncController.SyncNow)
		router.GET("/settings/obsidian/status", obsidianSyncController.GetStatus)
	}

	// Readwise sync settings routes (if SettingsStore and ReadwiseClient are available)
	if cfg.Core.SettingsStore != nil && cfg.Sync.ReadwiseClient != nil {
		readwiseSyncController := NewReadwiseSyncController(cfg.Core.SettingsStore, cfg.Sync.ReadwiseScheduler, cfg.Sync.ReadwiseClient)
		router.GET("/settings/readwise", readwiseSyncController.GetSettings)
		router.POST("/settings/readwise/save", readwiseSyncController.UpdateSettings)
		router.POST("/settings/readwise/reset", readwiseSyncController.ResetSettings)
		router.POST("/settings/readwise/validate-token", readwiseSyncController.ValidateToken)
		router.POST("/settings/readwise/sync-now", readwiseSyncController.SyncNow)
		router.GET("/settings/readwise/status", readwiseSyncController.GetStatus)
	}

	// Audit log routes (admin-only, requires AuditService)
	if cfg.Core.AuditService != nil {
		auditController := NewAuditController(cfg.Core.AuditService)
		router.GET("/audit", auditController.AuditLogPage)
		router.GET("/api/audit", auditController.GetAuditEvents)
	}

	return router
}
