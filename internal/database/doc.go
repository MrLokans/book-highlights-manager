// Package database provides the data access layer for the application.
//
// # Architecture
//
// The database layer is organized into domain-specific sub-packages:
//
//	database/
//	├── database.go      # Connection setup, migrations, source seeding
//	├── books/           # Book and highlight CRUD operations
//	├── tags/            # Tag management and associations
//	├── vocabulary/      # Vocabulary word management
//	├── favourites/      # Favourite highlight tracking
//	├── sync/            # Sync progress tracking
//	├── settings/        # Application settings
//	└── users/           # User management
//
// # Using Sub-packages
//
// Each sub-package provides a Repository type with domain-specific operations:
//
//	// Initialize database connection
//	db, err := database.NewDatabase("./app.db")
//
//	// Create domain-specific repositories
//	booksRepo := books.NewRepository(db.DB)
//	tagsRepo := tags.NewRepository(db.DB)
//	vocabRepo := vocabulary.NewRepository(db.DB)
//
//	// Use repositories
//	book, err := booksRepo.GetBookByID(123)
//	tags, err := tagsRepo.GetTagsForUser(userID)
//
// # Interface Implementations
//
// Each sub-package implements specific interfaces:
//
//   - books.Repository: implements services.BookReader (partial)
//   - tags.Repository: implements http.TagStore
//   - vocabulary.Repository: implements http.VocabularyStore
//   - favourites.Repository: implements http.FavouritesStore
//   - sync.Repository: implements metadata.ProgressReporter
//
// # Legacy Compatibility
//
// The main Database struct in database.go maintains all original methods
// for backward compatibility. New code should prefer using sub-packages
// directly for clearer dependencies and smaller interfaces.
//
// # Adding a New Domain
//
// To add a new domain (e.g., analytics):
//
//  1. Create a new sub-package: internal/database/analytics/
//  2. Define a Repository struct with a *gorm.DB field
//  3. Add NewRepository(db *gorm.DB) constructor
//  4. Implement the required interface
//  5. Add compile-time interface check: var _ SomeInterface = (*Repository)(nil)
package database
