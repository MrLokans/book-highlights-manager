package entrypoint

import (
	"context"
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
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/covers"
	"github.com/mrlokans/assistant/internal/database"
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

	if not := cfg.Obsidian.VaultDir == ""; not {
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
	coverCacheDir := filepath.Join(filepath.Dir(cfg.Database.Path), "covers")
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
	}

	router := http_controllers.NewRouter(routerCfg)

	// Shutdown callback for graceful cleanup
	onShutdown := func(ctx context.Context) {
		if taskClient != nil && taskCtxCancel != nil {
			taskClient.Stop(ctx)
			taskCtxCancel()
		}
	}

	Serve(router, cfg, onShutdown)
}
