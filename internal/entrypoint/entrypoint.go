// Package entrypoint handles application initialization, HTTP server startup, and graceful shutdown.
package entrypoint

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/analytics"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/covers"
	"github.com/mrlokans/assistant/internal/database"
	auditdb "github.com/mrlokans/assistant/internal/database/audit"
	"github.com/mrlokans/assistant/internal/demo"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/exporters"
	http_controllers "github.com/mrlokans/assistant/internal/http"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/mrlokans/assistant/internal/oauth2"
	"github.com/mrlokans/assistant/internal/oauth2/providers"
	"github.com/mrlokans/assistant/internal/readwise"
	"github.com/mrlokans/assistant/internal/scheduler"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/mrlokans/assistant/internal/tasks"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

// ShutdownFunc is called during graceful shutdown to clean up resources.
type ShutdownFunc func(ctx context.Context)

// Serve starts the HTTP server and blocks until a shutdown signal is received.
func Serve(router *gin.Engine, cfg *config.Config, onShutdown ShutdownFunc) {
	if cfg.Token == "" {
		log.Printf("WARNING: Readwise token is not set. Readwise import endpoint will be disabled. Set 'READWISE_TOKEN' environment variable to enable.")
	}

	// Export directory is now optional - only validate if configured
	if cfg.ExportDir != "" {
		log.Printf("Checking export directory: %s\n", cfg.ExportDir)

		// Check export dir exists as is a directory
		if _, err := os.Stat(cfg.ExportDir); os.IsNotExist(err) {
			log.Fatalf("Export directory %s does not exist", cfg.ExportDir)
			return
		}
		log.Printf("Export directory %s exists\n", cfg.ExportDir)

		// Check export dir is writable by touching and removing an empty file
		testFile := fmt.Sprintf("%s/.assistant", cfg.ExportDir)
		_, err := os.Create(filepath.Clean(testFile))
		if err != nil {
			log.Fatalf("Export directory %s is not writable", cfg.ExportDir)
			return
		}
		if removeErr := os.Remove(testFile); removeErr != nil {
			log.Printf("WARNING: Could not remove the test file from the export directory %s", cfg.ExportDir)
		}
	} else {
		log.Printf("WARNING: Obsidian export directory not configured. Markdown export will be disabled. Set 'OBSIDIAN_EXPORT_DIR' or configure via Settings UI.")
	}

	timeout := time.Duration(cfg.ShutdownTimeoutInSeconds) * time.Second

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Printf("Starting server at http://%s:%d\n", cfg.Host, cfg.Port)
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Graceful shutdown
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 1 second.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscanll.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("Shutdown Server, waiting %v before killing\n", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// Call shutdown callback first (e.g., to stop task queue)
	if onShutdown != nil {
		onShutdown(ctx)
	}

	if err := srv.Shutdown(ctx); err != nil {
		cancel()
		log.Fatal("Server Shutdown:", err)
	}

	cancel()
	log.Println("Server exiting")
}

// Run initialises all application components and starts the server.
func Run(cfg *config.Config, version string) {
	log.Printf("Starting Assistant v%s", version)

	demoMiddleware, demoCleanup := initDemo(cfg)

	// Initialize database
	db, err := database.NewDatabase(cfg.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	// Create the combined database + markdown exporter
	// It implements both BookReader and BookExporter interfaces
	exporter := exporters.NewDatabaseMarkdownExporter(
		db,
		cfg.ExportDir,
	)

	// Create audit service for logging application events
	auditRepo := auditdb.NewRepository(db.DB)
	auditService := audit.NewService(auditRepo)

	// Create cover cache for locally caching book covers
	// In demo mode with embedded assets, use the extracted covers path
	coverCacheDir := cfg.CoversPath
	if coverCacheDir == "" {
		coverCacheDir = filepath.Join(filepath.Dir(cfg.Path), "covers")
	}
	coverCache, err := covers.NewCache(coverCacheDir)
	if err != nil {
		log.Printf("WARNING: Failed to initialize cover cache: %v", err)
	} else {
		log.Printf("Cover cache initialized at %s", coverCacheDir)
	}

	// Create metadata enricher for book enrichment from OpenLibrary
	openLibraryClient := metadata.NewOpenLibraryClient()
	metadataUpdater := database.NewMetadataUpdater(db)
	metadataEnricher := metadata.NewEnricher(openLibraryClient, metadataUpdater)

	// Create progress reporter for tracking bulk sync operations
	syncProgress := database.NewMetadataSyncProgress(db)
	metadataEnricher.SetProgressReporter(syncProgress)

	// Connect cover cache to enricher for invalidation on metadata refresh
	if coverCache != nil {
		metadataEnricher.SetCoverInvalidator(coverCache)
	}

	// Create dictionary client for vocabulary enrichment
	dictClient := dictionary.NewFreeDictionaryClient()

	// Create Plausible analytics store
	plausibleStore := analytics.NewPlausibleStore(db, cfg.Plausible)

	// Create settings store for persistent settings
	settingsStore := settingsstore.New(db)

	// Create Obsidian sync scheduler
	obsidianScheduler := scheduler.NewObsidianSyncScheduler(db, settingsStore, auditService)

	// Create Readwise client and sync scheduler
	readwiseClient := readwise.NewClient()
	readwiseSyncScheduler := scheduler.NewReadwiseSyncScheduler(db, settingsStore, readwiseClient, auditService)

	oauth2Scheduler := initOAuth2(cfg, auditService)

	taskClient, taskCtxCancel, taskCleanup := initTaskQueue(cfg, metadataEnricher, db, dictClient, auditService)
	if taskCleanup != nil {
		defer taskCleanup()
	}

	authService, authMiddleware, sessionManager, csrfSecret := initAuth(cfg, db)

	// Build router configuration with all dependencies
	routerCfg := http_controllers.RouterConfig{
		BookReader:             exporter,
		BookExporter:           exporter,
		Database:               db,
		AuditService:           auditService,
		TagStore:               db,
		DeleteStore:            db,
		FavouritesStore:        db,
		VocabularyStore:        db,
		DictionaryClient:       dictClient,
		ReadwiseToken:          cfg.Token,
		TemplatesPath:          cfg.TemplatesPath,
		StaticPath:             cfg.StaticPath,
		DatabasePath:           cfg.Path,
		DropboxAppKey:          cfg.AppKey,
		MoonReaderDropboxPath:  cfg.DropboxPath,
		MoonReaderDatabasePath: cfg.DatabasePath,
		MoonReaderOutputDir:    cfg.OutputDir,
		Version:                version,
		MetadataEnricher:       metadataEnricher,
		SyncProgress:           syncProgress,
		CoverCache:             coverCache,
		TaskClient:             taskClient,
		TaskWorkers:            cfg.Workers,
		AuthService:            authService,
		AuthMiddleware:         authMiddleware,
		SessionManager:         sessionManager,
		AuthConfig:             cfg.Auth,
		CSRFSecret:             csrfSecret,
		SecureCookies:          cfg.SecureCookies,
		DemoMiddleware:         demoMiddleware,
		PlausibleStore:         plausibleStore,
		PlausibleConfig:        cfg.Plausible,
		SettingsStore:          settingsStore,
		ObsidianSyncScheduler:  obsidianScheduler,
		ReadwiseSyncScheduler:  readwiseSyncScheduler,
		ReadwiseClient:         readwiseClient,
	}

	router := http_controllers.NewRouter(routerCfg)

	// Start Obsidian sync scheduler if enabled
	if err := obsidianScheduler.Start(context.Background()); err != nil {
		log.Printf("WARNING: Failed to start Obsidian sync scheduler: %v", err)
	}

	// Start Readwise sync scheduler if enabled
	if err := readwiseSyncScheduler.Start(context.Background()); err != nil {
		log.Printf("WARNING: Failed to start Readwise sync scheduler: %v", err)
	}

	// Start OAuth2 token refresh scheduler
	var oauth2Ctx context.Context
	var oauth2Cancel context.CancelFunc
	if oauth2Scheduler != nil {
		oauth2Ctx, oauth2Cancel = context.WithCancel(context.Background())
		go oauth2Scheduler.Start(oauth2Ctx)
	}

	// Shutdown callback for graceful cleanup
	onShutdown := func(ctx context.Context) {
		// Stop Obsidian sync scheduler
		obsidianScheduler.Stop()

		// Stop Readwise sync scheduler
		readwiseSyncScheduler.Stop()

		// Stop OAuth2 token refresh scheduler
		if oauth2Scheduler != nil && oauth2Cancel != nil {
			oauth2Scheduler.Stop()
			oauth2Cancel()
		}

		if taskClient != nil && taskCtxCancel != nil {
			taskClient.Stop(ctx)
			taskCtxCancel()
		}
		if demoCleanup != nil {
			demoCleanup()
		}
	}

	Serve(router, cfg, onShutdown)
}

func initOAuth2(cfg *config.Config, auditService *audit.Service) *oauth2.RefreshScheduler {
	if !cfg.RefreshEnabled || cfg.AppKey == "" {
		return nil
	}

	providers.RegisterDropbox(cfg.AppKey)
	tokenStore, err := tokenstore.New(tokenstore.Config{DatabasePath: cfg.Path})
	if err != nil {
		log.Printf("WARNING: Failed to initialize OAuth2 token store: %v", err)
		return nil
	}

	s := oauth2.NewRefreshScheduler(tokenStore, oauth2.DefaultRegistry, oauth2.RefreshConfig{
		Enabled:       cfg.RefreshEnabled,
		CheckInterval: cfg.CheckInterval,
		RefreshMargin: cfg.RefreshMargin,
	}, auditService)
	log.Printf("OAuth2 token refresh scheduler initialized")
	return s
}

func initTaskQueue(cfg *config.Config, enricher *metadata.Enricher, db *database.Database, dictClient dictionary.Client, auditService *audit.Service) (*tasks.Client, context.CancelFunc, func()) {
	if !cfg.Tasks.Enabled {
		return nil, nil, nil
	}

	taskCfg := tasks.Config{
		Workers:           cfg.Workers,
		MaxRetries:        cfg.MaxRetries,
		RetryDelay:        cfg.RetryDelay,
		TaskTimeout:       cfg.TaskTimeout,
		ReleaseAfter:      cfg.ReleaseAfter,
		CleanupInterval:   cfg.CleanupInterval,
		RetentionDuration: cfg.RetentionDuration,
	}

	client, err := tasks.NewClient(cfg.Path, taskCfg)
	if err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}

	client.Register(
		tasks.NewEnrichBookQueue(enricher),
		tasks.NewEnrichAllBooksQueue(enricher),
		tasks.NewCleanupOrphanTagsQueue(db),
		tasks.NewEnrichWordQueue(db, dictClient),
		tasks.NewEnrichAllPendingWordsQueue(db, dictClient),
		tasks.NewCleanupAuditEventsQueue(auditService),
	)

	ctx, cancel := context.WithCancel(context.Background())
	go client.Start(ctx)

	cleanup := func() {
		if err := client.Close(); err != nil {
			log.Printf("Error closing task client: %v", err)
		}
	}
	return client, cancel, cleanup
}

func initDemo(cfg *config.Config) (*demo.Middleware, func()) {
	if !cfg.Demo.Enabled {
		return nil, nil
	}

	log.Printf("Demo mode enabled - write operations will be blocked")
	demoMiddleware := demo.NewMiddleware(true)

	if !cfg.UseEmbedded || !demo.HasEmbeddedAssets() {
		if cfg.UseEmbedded {
			log.Printf("Warning: DEMO_USE_EMBEDDED is true but no embedded assets found. Using file paths.")
		}
		return demoMiddleware, nil
	}

	tempDir, err := os.MkdirTemp("", "assistant-demo-*")
	if err != nil {
		log.Fatalf("Failed to create temp directory for demo assets: %v", err)
	}

	dbPath, coversPath, vaultPath, err := demo.ExtractAssets(tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir) //nolint:gosec // best-effort cleanup
		log.Fatalf("Failed to extract embedded demo assets: %v", err)
	}

	log.Printf("Extracted embedded demo assets to %s", tempDir)
	cfg.Path = dbPath
	cfg.DBPath = dbPath
	cfg.CoversPath = coversPath
	cfg.ExportDir = vaultPath

	cleanup := func() {
		log.Printf("Cleaning up demo assets from %s", tempDir)
		_ = os.RemoveAll(tempDir) //nolint:gosec // best-effort cleanup
	}
	return demoMiddleware, cleanup
}

func initAuth(cfg *config.Config, db *database.Database) (*auth.Service, *auth.Middleware, *auth.SessionManager, []byte) {
	if cfg.Mode != config.AuthModeLocal {
		log.Printf("Authentication mode: none (no authentication required)")
		return nil, nil, nil, nil
	}

	log.Printf("Authentication mode: local")
	authService := auth.NewService(db.DB, cfg.Auth)

	sqlDB, err := db.DB.DB()
	if err != nil {
		log.Fatalf("Failed to get SQL DB for sessions: %v", err)
	}

	sessionManager, err := auth.NewSessionManager(sqlDB, cfg.Auth)
	if err != nil {
		log.Fatalf("Failed to initialize session manager: %v", err)
	}

	authMiddleware := auth.NewMiddleware(authService, sessionManager, cfg.Auth)
	csrfSecret := resolveCSRFSecret(cfg)

	hasUsers, _ := authService.HasUsers()
	if !hasUsers {
		log.Printf("No users found. Visit /setup to create an administrator account.")
	}

	return authService, authMiddleware, sessionManager, csrfSecret
}

func resolveCSRFSecret(cfg *config.Config) []byte {
	if cfg.SessionSecret != "" {
		secret, err := hex.DecodeString(cfg.SessionSecret)
		if err != nil {
			return []byte(cfg.SessionSecret)
		}
		return secret
	}

	secret, err := auth.GenerateSessionSecret()
	if err != nil {
		log.Fatalf("Failed to generate CSRF secret: %v", err)
	}
	csrfSecret, _ := hex.DecodeString(secret)
	log.Printf("Generated session secret (set AUTH_SESSION_SECRET to persist)")
	return csrfSecret
}
