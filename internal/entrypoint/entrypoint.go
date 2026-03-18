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
	booksdb "github.com/mrlokans/assistant/internal/database/books"
	favouritesdb "github.com/mrlokans/assistant/internal/database/favourites"
	settingsdb "github.com/mrlokans/assistant/internal/database/settings"
	sourcesdb "github.com/mrlokans/assistant/internal/database/sources"
	syncdb "github.com/mrlokans/assistant/internal/database/sync"
	tagsdb "github.com/mrlokans/assistant/internal/database/tags"
	vocabularydb "github.com/mrlokans/assistant/internal/database/vocabulary"
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

const readHeaderTimeout = 10 * time.Second

// ShutdownFunc is called during graceful shutdown to clean up resources.
type ShutdownFunc func(ctx context.Context)

// repositories groups all per-domain database repositories.
type repositories struct {
	sources    *sourcesdb.Repository
	books      *booksdb.Repository
	tags       *tagsdb.Repository
	favourites *favouritesdb.Repository
	vocabulary *vocabularydb.Repository
	settings   *settingsdb.Repository
	sync       *syncdb.Repository
	audit      *auditdb.Repository
}

// services groups application-level services built from repositories.
type services struct {
	exporter          *exporters.DatabaseMarkdownExporter
	auditService      *audit.Service
	coverCache        *covers.Cache
	metadataEnricher  *metadata.Enricher
	syncProgress      *database.MetadataSyncProgress
	dictClient        dictionary.Client
	plausibleStore    *analytics.PlausibleStore
	settingsStore     *settingsstore.SettingsStore
	readwiseClient    *readwise.Client
	obsidianScheduler *scheduler.ObsidianSyncScheduler
	readwiseScheduler *scheduler.ReadwiseSyncScheduler
}

func initRepositories(db *database.Database) *repositories {
	sourcesRepo := sourcesdb.NewRepository(db.DB)
	return &repositories{
		sources:    sourcesRepo,
		books:      booksdb.NewRepository(db.DB, sourcesRepo),
		tags:       tagsdb.NewRepository(db.DB),
		favourites: favouritesdb.NewRepository(db.DB),
		vocabulary: vocabularydb.NewRepository(db.DB),
		settings:   settingsdb.NewRepository(db.DB),
		sync:       syncdb.NewRepository(db.DB),
		audit:      auditdb.NewRepository(db.DB),
	}
}

func initServices(cfg *config.Config, repos *repositories) *services {
	svc := &services{}

	svc.exporter = exporters.NewDatabaseMarkdownExporter(repos.books, cfg.ExportDir)
	svc.auditService = audit.NewService(repos.audit)

	svc.coverCache = initCoverCache(cfg)

	openLibraryClient := metadata.NewOpenLibraryClient()
	metadataUpdater := database.NewMetadataUpdater(repos.books)
	svc.metadataEnricher = metadata.NewEnricher(openLibraryClient, metadataUpdater)

	svc.syncProgress = database.NewMetadataSyncProgress(repos.sync)
	svc.metadataEnricher.SetProgressReporter(svc.syncProgress)
	if svc.coverCache != nil {
		svc.metadataEnricher.SetCoverInvalidator(svc.coverCache)
	}

	svc.dictClient = dictionary.NewFreeDictionaryClient()
	svc.plausibleStore = analytics.NewPlausibleStore(repos.settings, cfg.Plausible)
	svc.settingsStore = settingsstore.New(repos.settings)
	svc.readwiseClient = readwise.NewClient()
	svc.obsidianScheduler = scheduler.NewObsidianSyncScheduler(repos.books, repos.vocabulary, svc.settingsStore, svc.auditService)
	svc.readwiseScheduler = scheduler.NewReadwiseSyncScheduler(repos.books, repos.sources, svc.settingsStore, svc.readwiseClient, svc.auditService)

	return svc
}

func initCoverCache(cfg *config.Config) *covers.Cache {
	coverCacheDir := cfg.CoversPath
	if coverCacheDir == "" {
		coverCacheDir = filepath.Join(filepath.Dir(cfg.Path), "covers")
	}
	cache, err := covers.NewCache(coverCacheDir)
	if err != nil {
		log.Printf("WARNING: Failed to initialize cover cache: %v", err)
		return nil
	}
	log.Printf("Cover cache initialized at %s", coverCacheDir)
	return cache
}

func buildRouterConfig(cfg *config.Config, version string, db *database.Database, repos *repositories, svc *services,
	taskClient *tasks.Client, sharedTokenStore *tokenstore.TokenStore, demoMiddleware *demo.Middleware,
	authService *auth.Service, authMiddleware *auth.Middleware, sessionManager *auth.SessionManager, csrfSecret []byte,
) http_controllers.RouterConfig {
	return http_controllers.RouterConfig{
		BookReader:             repos.books,
		BookExporter:           svc.exporter,
		Pinger:                 db,
		AuditService:           svc.auditService,
		TagStore:               repos.tags,
		DeleteStore:            repos.books,
		FavouritesStore:        repos.favourites,
		VocabularyStore:        repos.vocabulary,
		DictionaryClient:       svc.dictClient,
		ReadwiseToken:          cfg.Token,
		TemplatesPath:          cfg.TemplatesPath,
		StaticPath:             cfg.StaticPath,
		TokenStore:             sharedTokenStore,
		DropboxAppKey:          cfg.AppKey,
		MoonReaderDropboxPath:  cfg.DropboxPath,
		MoonReaderDatabasePath: cfg.DatabasePath,
		MoonReaderOutputDir:    cfg.OutputDir,
		Version:                version,
		MetadataEnricher:       svc.metadataEnricher,
		SyncProgress:           svc.syncProgress,
		CoverCache:             svc.coverCache,
		TaskClient:             taskClient,
		TaskWorkers:            cfg.Workers,
		AuthService:            authService,
		AuthMiddleware:         authMiddleware,
		SessionManager:         sessionManager,
		AuthConfig:             cfg.Auth,
		CSRFSecret:             csrfSecret,
		SecureCookies:          cfg.SecureCookies,
		DemoMiddleware:         demoMiddleware,
		PlausibleStore:         svc.plausibleStore,
		SettingsStore:          svc.settingsStore,
		ObsidianSyncScheduler:  svc.obsidianScheduler,
		ReadwiseSyncScheduler:  svc.readwiseScheduler,
		ReadwiseClient:         svc.readwiseClient,
	}
}

func startSchedulers(svc *services, oauth2Scheduler *oauth2.RefreshScheduler) context.CancelFunc {
	if err := svc.obsidianScheduler.Start(context.Background()); err != nil {
		log.Printf("WARNING: Failed to start Obsidian sync scheduler: %v", err)
	}
	if err := svc.readwiseScheduler.Start(context.Background()); err != nil {
		log.Printf("WARNING: Failed to start Readwise sync scheduler: %v", err)
	}

	var oauth2Cancel context.CancelFunc
	if oauth2Scheduler != nil {
		var oauth2Ctx context.Context
		oauth2Ctx, oauth2Cancel = context.WithCancel(context.Background())
		go oauth2Scheduler.Start(oauth2Ctx)
	}
	return oauth2Cancel
}

func buildShutdownFunc(svc *services, oauth2Scheduler *oauth2.RefreshScheduler, oauth2Cancel context.CancelFunc,
	taskClient *tasks.Client, taskCtxCancel context.CancelFunc, demoCleanup func(),
) ShutdownFunc {
	return func(ctx context.Context) {
		svc.obsidianScheduler.Stop()
		svc.readwiseScheduler.Stop()

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
}

// Run initialises all application components and starts the server.
func Run(cfg *config.Config, version string) {
	log.Printf("Starting Assistant v%s", version)

	demoMiddleware, demoCleanup := initDemo(cfg)

	db, err := database.NewDatabase(cfg.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	repos := initRepositories(db)
	svc := initServices(cfg, repos)

	oauth2Scheduler := initOAuth2(cfg, svc.auditService)
	taskClient, taskCtxCancel, taskCleanup := initTaskQueue(cfg, svc.metadataEnricher, repos.tags, repos.vocabulary, svc.dictClient, svc.auditService)
	if taskCleanup != nil {
		defer taskCleanup()
	}

	authService, authMiddleware, sessionManager, csrfSecret := initAuth(cfg, db)
	sharedTokenStore := initTokenStore(cfg)

	routerCfg := buildRouterConfig(cfg, version, db, repos, svc, taskClient, sharedTokenStore, demoMiddleware, authService, authMiddleware, sessionManager, csrfSecret)
	router := http_controllers.NewRouter(routerCfg)

	oauth2Cancel := startSchedulers(svc, oauth2Scheduler)
	onShutdown := buildShutdownFunc(svc, oauth2Scheduler, oauth2Cancel, taskClient, taskCtxCancel, demoCleanup)

	Serve(router, cfg, onShutdown)
}

// Serve starts the HTTP server and blocks until a shutdown signal is received.
func Serve(router *gin.Engine, cfg *config.Config, onShutdown ShutdownFunc) {
	if cfg.Token == "" {
		log.Printf("WARNING: Readwise token is not set. Readwise import endpoint will be disabled. Set 'READWISE_TOKEN' environment variable to enable.")
	}

	if cfg.ExportDir != "" {
		validateExportDir(cfg.ExportDir)
	} else {
		log.Printf("WARNING: Obsidian export directory not configured. Markdown export will be disabled. Set 'OBSIDIAN_EXPORT_DIR' or configure via Settings UI.")
	}

	timeout := time.Duration(cfg.ShutdownTimeoutInSeconds) * time.Second

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		fmt.Printf("Starting server at http://%s:%d\n", cfg.Host, cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("Shutdown Server, waiting %v before killing\n", timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

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

func validateExportDir(exportDir string) {
	log.Printf("Checking export directory: %s\n", exportDir)

	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		log.Fatalf("Export directory %s does not exist", exportDir)
		return
	}
	log.Printf("Export directory %s exists\n", exportDir)

	testFile := fmt.Sprintf("%s/.assistant", exportDir)
	_, err := os.Create(filepath.Clean(testFile))
	if err != nil {
		log.Fatalf("Export directory %s is not writable", exportDir)
		return
	}
	if removeErr := os.Remove(testFile); removeErr != nil {
		log.Printf("WARNING: Could not remove the test file from the export directory %s", exportDir)
	}
}

func initTokenStore(cfg *config.Config) *tokenstore.TokenStore {
	if cfg.AppKey == "" {
		return nil
	}
	ts, err := tokenstore.New(tokenstore.Config{DatabasePath: cfg.Path})
	if err != nil {
		log.Printf("WARNING: Failed to initialize shared token store: %v", err)
		return nil
	}
	return ts
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

func initTaskQueue(cfg *config.Config, enricher *metadata.Enricher, tagsCleaner tasks.OrphanTagsCleaner, wordEnricher tasks.WordEnricher, dictClient dictionary.Client, auditService *audit.Service) (*tasks.Client, context.CancelFunc, func()) {
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
		tasks.NewCleanupOrphanTagsQueue(tagsCleaner),
		tasks.NewEnrichWordQueue(wordEnricher, dictClient),
		tasks.NewEnrichAllPendingWordsQueue(wordEnricher, dictClient),
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
