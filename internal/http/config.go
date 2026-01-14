package http

import (
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/covers"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/mrlokans/assistant/internal/services"
	"github.com/mrlokans/assistant/internal/tasks"
)

// RouterConfig contains all dependencies and configuration needed
// to create the HTTP router. This replaces the long parameter list
// in NewRouter for better maintainability.
type RouterConfig struct {
	// Core dependencies
	BookReader    exporters.BookReader
	BookExporter  exporters.BookExporter
	ImportService *services.ImportService
	Database      *database.Database
	Auditor       *audit.Auditor

	// Tag management
	TagStore TagStore

	// Authentication
	ReadwiseToken string

	// UI paths
	TemplatesPath string
	StaticPath    string

	// Database path (for settings controller that creates its own connection)
	DatabasePath string

	// Dropbox OAuth
	DropboxAppKey string

	// MoonReader configuration
	MoonReaderDropboxPath  string
	MoonReaderDatabasePath string
	MoonReaderOutputDir    string

	// Application info
	Version string

	// Metadata enrichment
	MetadataEnricher *metadata.Enricher

	// Sync progress tracking
	SyncProgress *database.MetadataSyncProgress

	// Cover caching
	CoverCache *covers.Cache

	// Delete operations
	DeleteStore DeleteStore

	// Favourites operations
	FavouritesStore FavouritesStore

	// Task queue client (optional)
	TaskClient  *tasks.Client
	TaskWorkers int

	// Vocabulary operations
	VocabularyStore  VocabularyStore
	DictionaryClient dictionary.Client
}
