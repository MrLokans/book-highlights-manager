// Package interfaces documents the core abstractions used throughout the application.
//
// This package consolidates interface documentation to help code agents understand
// extension points and how to implement new functionality.
//
// # Interface Categories
//
// The application uses several categories of interfaces:
//
// ## Data Access Interfaces
//
//   - BookReader: Read-only access to books (internal/services/interfaces.go)
//   - BookExporter: Persist books to storage (internal/services/interfaces.go)
//   - TagStore: Tag management (internal/http/tags.go)
//   - VocabularyStore: Word management (internal/http/vocabulary.go)
//   - FavouritesStore: Favourite tracking (internal/http/favourites.go)
//   - DeleteStore: Deletion operations (internal/http/delete.go)
//
// ## External Service Interfaces
//
//   - MetadataProvider: Book metadata from external APIs (internal/metadata/enricher.go)
//   - DictionaryClient: Word definitions (internal/dictionary/client.go)
//
// ## Progress Tracking Interfaces
//
//   - ProgressReporter: Sync progress reporting (internal/metadata/enricher.go)
//
// # Adding a New Import Source
//
// To add support for a new highlight import source:
//
//  1. Create converter in internal/importers/
//
//     type KoboHighlight struct {
//         Text      string `json:"text"`
//         BookTitle string `json:"volumeTitle"`
//     }
//
//     type KoboConverter struct {
//         Highlights []KoboHighlight
//     }
//
//     func (c *KoboConverter) Convert() ([]importers.RawHighlight, importers.Source) {
//         // Transform to common format
//     }
//
//     var _ importers.Converter = (*KoboConverter)(nil)
//
//  2. Create HTTP handler in internal/http/
//
//     type KoboImportController struct {
//         pipeline *importers.Pipeline
//     }
//
//     func (c *KoboImportController) Import(ctx *gin.Context) {
//         converter := importers.NewKoboConverter(req.Highlights)
//         result, err := c.pipeline.Import(converter)
//     }
//
//  3. Register route in router.go
//
// # Adding a New Metadata Provider
//
// To add a new source of book metadata (e.g., Google Books):
//
//  1. Implement MetadataProvider in internal/metadata/
//
//     type GoogleBooksClient struct {
//         apiKey     string
//         httpClient *http.Client
//     }
//
//     func (c *GoogleBooksClient) SearchByISBN(ctx context.Context, isbn string) (*BookMetadata, error)
//     func (c *GoogleBooksClient) SearchByTitle(ctx context.Context, title, author string) (*BookMetadata, error)
//
//     var _ MetadataProvider = (*GoogleBooksClient)(nil)
//
//  2. Add to enricher in entrypoint.go
//
// # Adding a New Dictionary Provider
//
// To add a new word definition source:
//
//  1. Implement DictionaryClient in internal/dictionary/
//
//     type MerriamWebsterClient struct {
//         apiKey string
//     }
//
//     func (c *MerriamWebsterClient) GetDefinitions(ctx context.Context, word string) ([]Definition, error)
//
//     var _ Client = (*MerriamWebsterClient)(nil)
//
//  2. Configure in entrypoint.go
//
// # Adding a New Database Domain
//
// To add a new data domain (e.g., analytics):
//
//  1. Create sub-package: internal/database/analytics/
//
//  2. Define repository:
//
//     type Repository struct { db *gorm.DB }
//
//     func NewRepository(db *gorm.DB) *Repository
//
//  3. Implement interface methods
//
//  4. Add compile-time check:
//
//     var _ AnalyticsStore = (*Repository)(nil)
//
// # Compile-Time Interface Checks
//
// All implementations should include compile-time checks to ensure they satisfy
// their interfaces. This catches missing methods at compile time rather than runtime:
//
//	var _ SomeInterface = (*MyImplementation)(nil)
//
// This pattern is used throughout the codebase. See checks.go for examples.
package interfaces
