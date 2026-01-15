package http

import (
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/covers"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/demo"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/mrlokans/assistant/internal/services"
	"github.com/mrlokans/assistant/internal/tasks"
)

// RouterConfig contains all dependencies and configuration needed
// to create the HTTP router. Dependencies are organized by feature area
// for better maintainability and discoverability.
//
// Optional features: Set the corresponding field to nil to disable endpoints:
//   - TagStore: nil disables /api/tags/* endpoints
//   - DeleteStore: nil disables DELETE /api/books/* and /api/highlights/*
//   - FavouritesStore: nil disables /api/highlights/*/favourite endpoints
//   - VocabularyStore: nil disables /api/vocabulary/* endpoints
//   - MetadataEnricher: nil disables /api/books/:id/enrich endpoints
//   - CoverCache: nil disables /api/books/:id/cover endpoint
//   - TaskClient: nil disables /api/tasks/* endpoints
type RouterConfig struct {
	// --- Core Dependencies ---

	// BookReader provides read access to books and highlights.
	BookReader exporters.BookReader

	// BookExporter handles saving books to the database.
	BookExporter exporters.BookExporter

	// ImportService orchestrates import operations (optional, for future use).
	ImportService *services.ImportService

	// Database provides direct database access for health checks.
	Database *database.Database

	// Auditor logs incoming requests for debugging (optional).
	Auditor *audit.Auditor

	// --- Store Interfaces ---
	// Each store interface enables a feature area. Set to nil to disable.

	// TagStore provides tag CRUD operations.
	TagStore TagStore

	// DeleteStore provides soft/permanent delete operations.
	DeleteStore DeleteStore

	// FavouritesStore provides highlight favouriting operations.
	FavouritesStore FavouritesStore

	// VocabularyStore provides vocabulary word management.
	VocabularyStore VocabularyStore

	// --- Authentication ---

	// ReadwiseToken authenticates Readwise API import requests.
	ReadwiseToken string

	// DropboxAppKey enables Dropbox OAuth for MoonReader backup import.
	DropboxAppKey string

	// --- Paths ---

	// TemplatesPath is the directory containing HTML templates.
	TemplatesPath string

	// StaticPath is the directory containing static assets (CSS, JS, images).
	StaticPath string

	// DatabasePath is used by settings controller for its own connection.
	DatabasePath string

	// --- MoonReader Configuration ---

	// MoonReaderDropboxPath is the path to MoonReader backup in Dropbox.
	MoonReaderDropboxPath string

	// MoonReaderDatabasePath is the local path to MoonReader database.
	MoonReaderDatabasePath string

	// MoonReaderOutputDir is the output directory for processed highlights.
	MoonReaderOutputDir string

	// --- Metadata Enrichment ---

	// MetadataEnricher enriches books with OpenLibrary data (optional).
	MetadataEnricher *metadata.Enricher

	// SyncProgress tracks metadata sync progress.
	SyncProgress *database.MetadataSyncProgress

	// CoverCache caches book cover images (optional).
	CoverCache *covers.Cache

	// --- Background Tasks ---

	// TaskClient provides background task queue (optional).
	TaskClient *tasks.Client

	// TaskWorkers is the number of concurrent task workers.
	TaskWorkers int

	// --- Dictionary ---

	// DictionaryClient provides word definition lookups.
	DictionaryClient dictionary.Client

	// --- Application Info ---

	// Version is displayed in health check responses.
	Version string

	// --- Authentication ---

	// AuthService handles user authentication (optional, nil for no auth).
	AuthService *auth.Service

	// AuthMiddleware applies auth checks to routes (optional).
	AuthMiddleware *auth.Middleware

	// SessionManager handles session cookies (optional).
	SessionManager *auth.SessionManager

	// AuthConfig contains authentication configuration.
	AuthConfig config.Auth

	// CSRFSecret is the secret key for CSRF protection (required when auth enabled).
	CSRFSecret []byte

	// SecureCookies controls HTTPS-only cookies.
	SecureCookies bool

	// --- Demo Mode ---

	// DemoMiddleware blocks write operations in demo mode (optional).
	DemoMiddleware *demo.Middleware
}
