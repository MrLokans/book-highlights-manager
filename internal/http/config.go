package http

import (
	"github.com/mrlokans/assistant/internal/analytics"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/covers"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/demo"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/mrlokans/assistant/internal/readwise"
	"github.com/mrlokans/assistant/internal/scheduler"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/mrlokans/assistant/internal/tasks"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

// RouterConfig contains all dependencies and configuration needed
// to create the HTTP router. Dependencies are organized by feature area
// for better maintainability and discoverability.
//
// Optional features: Set the corresponding field to nil to disable endpoints:
//   - Stores.TagStore: nil disables /api/tags/* endpoints
//   - Stores.DeleteStore: nil disables DELETE /api/books/* and /api/highlights/*
//   - Stores.FavouritesStore: nil disables /api/highlights/*/favourite endpoints
//   - Stores.VocabularyStore: nil disables /api/vocabulary/* endpoints
//   - Metadata.Enricher: nil disables /api/books/:id/enrich endpoints
//   - Metadata.CoverCache: nil disables /api/books/:id/cover endpoint
//   - Tasks.Client: nil disables /api/tasks/* endpoints
type RouterConfig struct {
	Core     CoreDeps
	Stores   StoreDeps
	Auth     AuthDeps
	Import   ImportDeps
	Metadata MetadataDeps
	Tasks    TaskDeps
	Sync     SyncDeps
	UI       UIDeps
}

// CoreDeps contains essential application dependencies.
type CoreDeps struct {
	BookReader       exporters.BookReader
	BookExporter     exporters.BookExporter
	Pinger           DatabasePinger
	AuditService     *audit.Service
	DictionaryClient dictionary.Client
	Version          string
	DemoMiddleware   *demo.Middleware
	PlausibleStore   *analytics.PlausibleStore
	SettingsStore    *settingsstore.SettingsStore
}

// StoreDeps contains optional per-domain data store interfaces.
// Set any field to nil to disable the corresponding feature endpoints.
type StoreDeps struct {
	TagStore        TagStore
	DeleteStore     DeleteStore
	FavouritesStore FavouritesStore
	VocabularyStore VocabularyStore
}

// AuthDeps contains authentication and session management dependencies.
type AuthDeps struct {
	Service        *auth.Service
	Middleware     *auth.Middleware
	SessionManager *auth.SessionManager
	Config         config.Auth
	CSRFSecret     []byte
	SecureCookies  bool
}

// ImportDeps contains configuration for highlight import sources.
type ImportDeps struct {
	ReadwiseToken          string
	DropboxAppKey          string
	TokenStore             *tokenstore.TokenStore
	MoonReaderDropboxPath  string
	MoonReaderDatabasePath string
	MoonReaderOutputDir    string
}

// MetadataDeps contains book metadata enrichment dependencies.
type MetadataDeps struct {
	Enricher     *metadata.Enricher
	SyncProgress *database.MetadataSyncProgress
	CoverCache   *covers.Cache
}

// TaskDeps contains background task queue dependencies.
type TaskDeps struct {
	Client  *tasks.Client
	Workers int
}

// SyncDeps contains sync scheduler dependencies.
type SyncDeps struct {
	ObsidianScheduler *scheduler.ObsidianSyncScheduler
	ReadwiseScheduler *scheduler.ReadwiseSyncScheduler
	ReadwiseClient    *readwise.Client
}

// UIDeps contains paths for templates and static assets.
type UIDeps struct {
	TemplatesPath string
	StaticPath    string
}
