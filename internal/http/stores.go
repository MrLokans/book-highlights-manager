package http

import "github.com/mrlokans/assistant/internal/entities"

// This file consolidates all store interface definitions used by HTTP controllers.
// Each controller defines its own interface (Interface Segregation Principle),
// but this file provides a comprehensive view of all database operations needed.

// --- Entity Retrieval (shared across multiple controllers) ---

// BookGetter provides read access to books.
type BookGetter interface {
	GetBookByID(id uint) (*entities.Book, error)
}

// HighlightGetter provides read access to highlights.
type HighlightGetter interface {
	GetHighlightByID(id uint) (*entities.Highlight, error)
}

// --- Composite Interface ---

// Store combines all store interfaces for controllers that need broad access.
// Use this when a controller needs multiple store capabilities, or for testing.
type Store interface {
	// Books
	BookGetter
	GetAllBooks(userID uint) ([]entities.Book, error)
	GetBookByTitleAndAuthor(title, author string, userID uint) (*entities.Book, error)
	GetBookStats(userID uint) (totalBooks, totalHighlights int, err error)
	DeleteBook(id uint) error
	DeleteBookPermanently(id uint, userID uint) error

	// Highlights
	HighlightGetter
	DeleteHighlight(id uint) error
	DeleteHighlightPermanently(id uint, userID uint) error
	SetHighlightFavourite(highlightID uint, isFavourite bool) error
	GetFavouriteHighlights(userID uint, limit, offset int) ([]entities.Highlight, int64, error)
	GetFavouriteHighlightsByBook(bookID uint) ([]entities.Highlight, error)
	GetFavouriteCount(userID uint) (int64, error)

	// Tags
	CreateTag(name string, userID uint) (*entities.Tag, error)
	GetOrCreateTag(name string, userID uint) (*entities.Tag, error)
	GetTagsForUser(userID uint) ([]entities.Tag, error)
	SearchTags(query string, userID uint) ([]entities.Tag, error)
	GetTagByID(id uint) (*entities.Tag, error)
	DeleteTag(id uint) error
	DeleteOrphanTags() (int64, error)
	AddTagToBook(bookID, tagID uint) error
	RemoveTagFromBook(bookID, tagID uint) error
	AddTagToHighlight(highlightID, tagID uint) error
	RemoveTagFromHighlight(highlightID, tagID uint) error
	GetBooksByTag(tagID uint, userID uint) ([]entities.Book, error)

	// Vocabulary
	AddWord(word *entities.Word) error
	GetAllWords(userID uint, limit, offset int) ([]entities.Word, int64, error)
	GetWordByID(id uint) (*entities.Word, error)
	UpdateWord(word *entities.Word) error
	DeleteWord(id uint) error
	GetPendingWords(limit int) ([]entities.Word, error)
	SaveDefinitions(wordID uint, definitions []entities.WordDefinition) error
	UpdateWordStatus(id uint, status entities.WordStatus, errorMsg string) error
	GetWordsByHighlight(highlightID uint) ([]entities.Word, error)
	GetWordsByBook(bookID uint) ([]entities.Word, error)
	FindWordBySource(word, sourceBookTitle, sourceBookAuthor, sourceHighlightText string, userID uint) (*entities.Word, error)
	SearchWords(query string, userID uint, limit int) ([]entities.Word, error)
	GetVocabularyStats(userID uint) (total, pending, enriched, failed int64, err error)
	GetWordsByStatus(userID uint, status entities.WordStatus, limit, offset int) ([]entities.Word, int64, error)
}

// --- Interface Documentation ---
//
// Controller-specific interfaces (defined in their respective files):
//
// TagStore (tags.go):
//   - Full tag CRUD operations
//   - Book/highlight tag associations
//   - Tag search and suggestions
//
// DeleteStore (delete.go):
//   - Soft and permanent delete for books/highlights
//   - Entity retrieval for pre-delete checks
//
// FavouritesStore (favourites.go):
//   - Favourite toggle and retrieval
//   - Paginated favourite lists
//
// VocabularyStore (vocabulary.go):
//   - Word CRUD operations
//   - Definition management
//   - Enrichment status tracking
//
// These interfaces follow the Interface Segregation Principle:
// each controller only depends on the methods it actually uses.
