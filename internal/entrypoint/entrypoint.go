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
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/covers"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/demo"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/exporters"
	http_controllers "github.com/mrlokans/assistant/internal/http"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/mrlokans/assistant/internal/tasks"
)

// ShutdownFunc is called during graceful shutdown to clean up resources.
type ShutdownFunc func(ctx context.Context)

func Serve(router *gin.Engine, cfg *config.Config, onShutdown ShutdownFunc) {
	if cfg.Readwise.Token == "" {
		log.Printf("WARNING: Readwise token is not set. Readwise import endpoint will be disabled. Set 'READWISE_TOKEN' environment variable to enable.")
	}

	log.Printf("Checking vault directory: %s\n", cfg.Obsidian.VaultDir)

	if cfg.Obsidian.VaultDir == "" {
		log.Fatalf("Vault directory is not set")
		return
	}

	// Check export dir exists as is a directory
	if _, err := os.Stat(cfg.Obsidian.VaultDir); os.IsNotExist(err) {
		log.Fatalf("Vault directory %s does not exist", cfg.Obsidian.VaultDir)
		return
	} else {
		log.Printf("Vault directory %s exists\n", cfg.Obsidian.VaultDir)
	}

	// Check export dir is writable by touching and removing an empty file
	_, err := os.Create(fmt.Sprintf("%s/.assistant", cfg.Obsidian.VaultDir))

	// Defer the removal of the temp file file
	defer func() {
		err := os.Remove(fmt.Sprintf("%s/.assistant", cfg.Obsidian.VaultDir))
		if err != nil {
			log.Fatalf("Could not remove the test file from the vault directory %s", cfg.Obsidian.VaultDir)
			return
		}
	}()

	if err != nil {
		log.Fatalf("Vault directory %s is not writable", cfg.Obsidian.VaultDir)
		return
	}

	timeout := time.Duration(cfg.Global.ShutdownTimeoutInSeconds) * time.Second

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler: router,
	}

	go func() {
		fmt.Printf("Starting server at %s:%d\n", cfg.HTTP.Host, cfg.HTTP.Port)
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
	defer cancel()

	// Call shutdown callback first (e.g., to stop task queue)
	if onShutdown != nil {
		onShutdown(ctx)
	}

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}

	log.Println("Server exiting")
}

func Run(cfg *config.Config, version string) {
	log.Printf("Starting Assistant v%s", version)

	// Initialize demo mode middleware and extract embedded assets if needed
	var demoMiddleware *demo.Middleware
	var demoCleanup func()
	if cfg.Demo.Enabled {
		log.Printf("Demo mode enabled - write operations will be blocked")
		demoMiddleware = demo.NewMiddleware(true)

		// Extract embedded assets if configured and available
		if cfg.Demo.UseEmbedded && demo.HasEmbeddedAssets() {
			tempDir, err := os.MkdirTemp("", "assistant-demo-*")
			if err != nil {
				log.Fatalf("Failed to create temp directory for demo assets: %v", err)
			}

			dbPath, coversPath, vaultPath, err := demo.ExtractAssets(tempDir)
			if err != nil {
				os.RemoveAll(tempDir)
				log.Fatalf("Failed to extract embedded demo assets: %v", err)
			}

			log.Printf("Extracted embedded demo assets to %s", tempDir)
			log.Printf("  Database: %s", dbPath)
			log.Printf("  Covers: %s", coversPath)
			log.Printf("  Vault: %s", vaultPath)

			// Override config paths with extracted paths
			cfg.Database.Path = dbPath
			cfg.Demo.DBPath = dbPath
			cfg.Demo.CoversPath = coversPath
			cfg.Obsidian.VaultDir = vaultPath

			// Set up cleanup on shutdown
			demoCleanup = func() {
				log.Printf("Cleaning up demo assets from %s", tempDir)
				os.RemoveAll(tempDir)
			}
		} else if cfg.Demo.UseEmbedded {
			log.Printf("Warning: DEMO_USE_EMBEDDED is true but no embedded assets found. Using file paths.")
		}
	}

	// Initialize database
	db, err := database.NewDatabase(cfg.Database.Path)
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
		cfg.Obsidian.VaultDir,
		cfg.Obsidian.ExportPath,
	)

	// Create auditor for saving incoming JSON requests
	auditor := audit.NewAuditor(cfg.Audit.Dir)

	// Create cover cache for locally caching book covers
	// In demo mode with embedded assets, use the extracted covers path
	coverCacheDir := cfg.Demo.CoversPath
	if coverCacheDir == "" {
		coverCacheDir = filepath.Join(filepath.Dir(cfg.Database.Path), "covers")
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

	// Initialize task queue if enabled
	var taskClient *tasks.Client
	var taskCtxCancel context.CancelFunc
	if cfg.Tasks.Enabled {
		taskCfg := tasks.Config{
			Workers:           cfg.Tasks.Workers,
			MaxRetries:        cfg.Tasks.MaxRetries,
			RetryDelay:        cfg.Tasks.RetryDelay,
			TaskTimeout:       cfg.Tasks.TaskTimeout,
			ReleaseAfter:      cfg.Tasks.ReleaseAfter,
			CleanupInterval:   cfg.Tasks.CleanupInterval,
			RetentionDuration: cfg.Tasks.RetentionDuration,
		}

		taskClient, err = tasks.NewClient(cfg.Database.Path, taskCfg)
		if err != nil {
			log.Fatalf("Failed to initialize task queue: %v", err)
		}
		defer func() {
			if err := taskClient.Close(); err != nil {
				log.Printf("Error closing task client: %v", err)
			}
		}()

		// Register task queues
		taskClient.Register(
			tasks.NewEnrichBookQueue(metadataEnricher),
			tasks.NewEnrichAllBooksQueue(metadataEnricher),
			tasks.NewCleanupOrphanTagsQueue(db),
			tasks.NewEnrichWordQueue(db, dictClient),
			tasks.NewEnrichAllPendingWordsQueue(db, dictClient),
		)

		// Start task workers in background
		var taskCtx context.Context
		taskCtx, taskCtxCancel = context.WithCancel(context.Background())
		go taskClient.Start(taskCtx)
	}

	// Initialize authentication if enabled
	var authService *auth.Service
	var authMiddleware *auth.Middleware
	var sessionManager *auth.SessionManager
	var csrfSecret []byte

	if cfg.Auth.Mode == config.AuthModeLocal {
		log.Printf("Authentication mode: local")

		// Create auth service
		authService = auth.NewService(db.DB, cfg.Auth)

		// Get underlying SQL DB for session store
		sqlDB, err := db.DB.DB()
		if err != nil {
			log.Fatalf("Failed to get SQL DB for sessions: %v", err)
		}

		// Initialize session manager
		sessionManager, err = auth.NewSessionManager(sqlDB, cfg.Auth)
		if err != nil {
			log.Fatalf("Failed to initialize session manager: %v", err)
		}

		// Create auth middleware
		authMiddleware = auth.NewMiddleware(authService, sessionManager, cfg.Auth)

		// Generate or use configured CSRF secret
		if cfg.Auth.SessionSecret != "" {
			csrfSecret, err = hex.DecodeString(cfg.Auth.SessionSecret)
			if err != nil {
				// Not hex, use as raw bytes
				csrfSecret = []byte(cfg.Auth.SessionSecret)
			}
		} else {
			// Generate a secret
			secret, err := auth.GenerateSessionSecret()
			if err != nil {
				log.Fatalf("Failed to generate CSRF secret: %v", err)
			}
			csrfSecret, _ = hex.DecodeString(secret)
			log.Printf("Generated session secret (set AUTH_SESSION_SECRET to persist)")
		}

		// Check if setup is needed
		hasUsers, _ := authService.HasUsers()
		if !hasUsers {
			log.Printf("No users found. Visit /setup to create an administrator account.")
		}
	} else {
		log.Printf("Authentication mode: none (no authentication required)")
	}

	// Build router configuration with all dependencies
	routerCfg := http_controllers.RouterConfig{
		BookReader:             exporter,
		BookExporter:           exporter,
		Database:               db,
		Auditor:                auditor,
		TagStore:               db,
		DeleteStore:            db,
		FavouritesStore:        db,
		VocabularyStore:        db,
		DictionaryClient:       dictClient,
		ReadwiseToken:          cfg.Readwise.Token,
		TemplatesPath:          cfg.UI.TemplatesPath,
		StaticPath:             cfg.UI.StaticPath,
		DatabasePath:           cfg.Database.Path,
		DropboxAppKey:          cfg.Dropbox.AppKey,
		MoonReaderDropboxPath:  cfg.MoonReader.DropboxPath,
		MoonReaderDatabasePath: cfg.MoonReader.DatabasePath,
		MoonReaderOutputDir:    cfg.MoonReader.OutputDir,
		Version:                version,
		MetadataEnricher:       metadataEnricher,
		SyncProgress:           syncProgress,
		CoverCache:             coverCache,
		TaskClient:             taskClient,
		TaskWorkers:            cfg.Tasks.Workers,
		AuthService:            authService,
		AuthMiddleware:         authMiddleware,
		SessionManager:         sessionManager,
		AuthConfig:             cfg.Auth,
		CSRFSecret:             csrfSecret,
		SecureCookies:          cfg.Auth.SecureCookies,
		DemoMiddleware:         demoMiddleware,
	}

	router := http_controllers.NewRouter(routerCfg)

	// Shutdown callback for graceful cleanup
	onShutdown := func(ctx context.Context) {
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
