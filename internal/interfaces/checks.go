package interfaces

// This file contains compile-time interface implementation checks.
// These ensure that concrete types satisfy their interfaces at compile time,
// catching missing methods before runtime.
//
// To verify all checks pass: go build ./internal/interfaces/...

import (
	"github.com/mrlokans/assistant/internal/database/favourites"
	"github.com/mrlokans/assistant/internal/database/sync"
	"github.com/mrlokans/assistant/internal/database/tags"
	"github.com/mrlokans/assistant/internal/database/vocabulary"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/http"
	"github.com/mrlokans/assistant/internal/importers"
	"github.com/mrlokans/assistant/internal/metadata"
)

// =============================================================================
// Data Access Layer
// =============================================================================

// TagStore implementations
var _ http.TagStore = (*tags.Repository)(nil)

// VocabularyStore implementations
var _ http.VocabularyStore = (*vocabulary.Repository)(nil)

// FavouritesStore implementations
var _ http.FavouritesStore = (*favourites.Repository)(nil)

// BookReader/BookExporter implementations
var _ exporters.BookReader = (*exporters.DatabaseMarkdownExporter)(nil)
var _ exporters.BookExporter = (*exporters.DatabaseMarkdownExporter)(nil)

// =============================================================================
// External Services
// =============================================================================

// MetadataProvider implementations
var _ metadata.MetadataProvider = (*metadata.OpenLibraryClient)(nil)

// DictionaryClient implementations
var _ dictionary.Client = (*dictionary.FreeDictionaryClient)(nil)

// =============================================================================
// Progress Tracking
// =============================================================================

// ProgressReporter implementations
var _ metadata.ProgressReporter = (*sync.Repository)(nil)

// =============================================================================
// Import Pipeline
// =============================================================================

// Converter implementations
var _ importers.Converter = (*importers.ReadwiseConverter)(nil)
var _ importers.Converter = (*importers.ReadwiseCSVConverter)(nil)
var _ importers.Converter = (*importers.MoonReaderConverter)(nil)
