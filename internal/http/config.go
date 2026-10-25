package http

import (
	"github.com/mrlokans/assistant/internal/analytics"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/covers"
	booksdb "github.com/mrlokans/assistant/internal/database/books"
	favouritesdb "github.com/mrlokans/assistant/internal/database/favourites"
	tagsdb "github.com/mrlokans/assistant/internal/database/tags"
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

// RouterConfig contains all dependencies and configuration needed to create the HTTP router.
//
// Optional features: leave the field nil to disable its endpoints:
//   - TagStore: /api/tags/*
//   - DeleteStore: DELETE /api/books/* and /api/highlights/*
//   - FavouritesStore: /api/highlights/*/favourite
//   - VocabularyStore: /api/vocabulary/*
//   - Enricher: /api/books/:id/enrich
//   - CoverCache: /api/books/:id/cover
//   - TaskClient: /api/tasks/*
type RouterConfig struct {
	// Core
	BookReader       exporters.BookReader
	BookExporter     exporters.BookExporter
	Pinger           DatabasePinger
	AuditService     *audit.Service
	DictionaryClient dictionary.Client
	Version          string
	DemoMiddleware   *demo.Middleware
	PlausibleStore   *analytics.PlausibleStore
	SettingsStore    *settingsstore.SettingsStore

	// Stores
	BookLister      *booksdb.Repository
	TagStore        *tagsdb.Repository
	DeleteStore     DeleteStore
	FavouritesStore *favouritesdb.Repository
	VocabularyStore VocabularyStore

	// Auth
	AuthService    *auth.Service
	AuthMiddleware *auth.Middleware
	SessionManager *auth.SessionManager
	AuthConfig     config.Auth
	CSRFSecret     []byte
	SecureCookies  bool

	// Import
	ReadwiseToken         string
	DropboxAppKey         string
	TokenStore            *tokenstore.TokenStore
	MoonReaderDropboxPath string

	// Metadata
	Enricher     *metadata.Enricher
	SyncProgress SyncProgressReader
	CoverCache   *covers.Cache

	// Background tasks
	TaskClient  *tasks.Client
	TaskWorkers int

	// Sync
	ObsidianScheduler *scheduler.ObsidianSyncScheduler
	ReadwiseScheduler *scheduler.ReadwiseSyncScheduler
	ReadwiseClient    *readwise.Client

	// UI
	TemplatesPath string
	StaticPath    string
}
