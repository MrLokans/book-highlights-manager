package metadata

import (
	"context"
	"fmt"

	"github.com/mrlokans/assistant/internal/entities"
)

// MetadataProvider defines the interface for fetching book metadata.
type MetadataProvider interface {
	SearchByISBN(ctx context.Context, isbn string) (*BookMetadata, error)
	SearchByTitle(ctx context.Context, title, author string) (*BookMetadata, error)
}

// BookUpdater defines the interface for updating books in the database.
type BookUpdater interface {
	GetBookByID(id uint) (*entities.Book, error)
	UpdateBookMetadata(id uint, metadata BookUpdateFields) error
	GetBooksMissingMetadata() ([]entities.Book, error)
}

// CoverInvalidator defines the interface for invalidating cached covers.
type CoverInvalidator interface {
	InvalidateCover(bookID uint) error
}

// ProgressReporter reports sync progress updates.
type ProgressReporter interface {
	StartSync(totalItems int) error
	UpdateProgress(processed, succeeded, failed, skipped int, currentItem string) error
	CompleteSync(succeeded bool, errorMsg string) error
	IsSyncRunning() (bool, error)
}

// BookUpdateFields contains the fields that can be updated via enrichment.
type BookUpdateFields struct {
	ISBN            *string
	CoverURL        *string
	Publisher       *string
	PublicationYear *int
}

// EnrichmentResult contains the result of an enrichment operation.
type EnrichmentResult struct {
	Book          *entities.Book `json:"book"`
	FieldsUpdated []string       `json:"fields_updated"`
	Source        string         `json:"source"`
	SearchMethod  string         `json:"search_method"` // "isbn" or "title"
}

// Enricher handles book metadata enrichment from external sources.
type Enricher struct {
	provider         MetadataProvider
	db               BookUpdater
	coverInvalidator CoverInvalidator
	progressReporter ProgressReporter
}

// NewEnricher creates a new Enricher with the given metadata provider and database.
func NewEnricher(provider MetadataProvider, db BookUpdater) *Enricher {
	return &Enricher{
		provider: provider,
		db:       db,
	}
}

// SetCoverInvalidator sets the cover cache invalidator (optional).
func (e *Enricher) SetCoverInvalidator(invalidator CoverInvalidator) {
	e.coverInvalidator = invalidator
}

// SetProgressReporter sets the progress reporter for bulk operations (optional).
func (e *Enricher) SetProgressReporter(reporter ProgressReporter) {
	e.progressReporter = reporter
}

// EnrichBook fetches metadata for a book and updates it in the database.
// It tries ISBN first (if available), then falls back to title+author search.
func (e *Enricher) EnrichBook(ctx context.Context, bookID uint) (*EnrichmentResult, error) {
	book, err := e.db.GetBookByID(bookID)
	if err != nil {
		return nil, fmt.Errorf("get book: %w", err)
	}

	var metadata *BookMetadata
	var searchMethod string

	// Try ISBN first if available
	if book.ISBN != "" {
		metadata, err = e.provider.SearchByISBN(ctx, book.ISBN)
		if err == nil {
			searchMethod = "isbn"
		}
	}

	// Fall back to title+author search
	if metadata == nil {
		metadata, err = e.provider.SearchByTitle(ctx, book.Title, book.Author)
		if err != nil {
			return nil, fmt.Errorf("metadata search failed: %w", err)
		}
		searchMethod = "title"
	}

	// Apply metadata updates
	updates, fieldsUpdated := e.buildUpdates(book, metadata)

	if len(fieldsUpdated) > 0 {
		// Invalidate cached cover if cover URL changed
		if updates.CoverURL != nil && e.coverInvalidator != nil {
			_ = e.coverInvalidator.InvalidateCover(bookID)
		}

		if err := e.db.UpdateBookMetadata(bookID, updates); err != nil {
			return nil, fmt.Errorf("update book metadata: %w", err)
		}

		// Refresh book from database
		book, err = e.db.GetBookByID(bookID)
		if err != nil {
			return nil, fmt.Errorf("refresh book: %w", err)
		}
	}

	return &EnrichmentResult{
		Book:          book,
		FieldsUpdated: fieldsUpdated,
		Source:        "openlibrary",
		SearchMethod:  searchMethod,
	}, nil
}

// EnrichBookWithISBN searches by ISBN first, and if found, updates the book with ISBN and metadata.
// If ISBN search fails, falls back to title+author search.
func (e *Enricher) EnrichBookWithISBN(ctx context.Context, bookID uint, isbn string) (*EnrichmentResult, error) {
	book, err := e.db.GetBookByID(bookID)
	if err != nil {
		return nil, fmt.Errorf("get book: %w", err)
	}

	var metadata *BookMetadata
	var searchMethod string

	// Try ISBN search first
	metadata, err = e.provider.SearchByISBN(ctx, isbn)
	if err == nil && metadata != nil {
		searchMethod = "isbn"
		// ISBN search succeeded - ensure the provided ISBN is included in updates
		if metadata.ISBN == "" {
			metadata.ISBN = isbn
		}
	}

	// Fall back to title+author search if ISBN search failed
	if metadata == nil {
		metadata, err = e.provider.SearchByTitle(ctx, book.Title, book.Author)
		if err != nil {
			return nil, fmt.Errorf("metadata search failed: %w", err)
		}
		searchMethod = "title"
	}

	// Apply metadata updates
	updates, fieldsUpdated := e.buildUpdates(book, metadata)

	// If we searched by ISBN and found results, always save that ISBN
	if searchMethod == "isbn" && book.ISBN != isbn {
		updates.ISBN = &isbn
		if !contains(fieldsUpdated, "isbn") {
			fieldsUpdated = append(fieldsUpdated, "isbn")
		}
	}

	if len(fieldsUpdated) > 0 {
		// Invalidate cached cover if cover URL changed
		if updates.CoverURL != nil && e.coverInvalidator != nil {
			_ = e.coverInvalidator.InvalidateCover(bookID)
		}

		if err := e.db.UpdateBookMetadata(bookID, updates); err != nil {
			return nil, fmt.Errorf("update book metadata: %w", err)
		}

		// Refresh book from database
		book, err = e.db.GetBookByID(bookID)
		if err != nil {
			return nil, fmt.Errorf("refresh book: %w", err)
		}
	}

	return &EnrichmentResult{
		Book:          book,
		FieldsUpdated: fieldsUpdated,
		Source:        "openlibrary",
		SearchMethod:  searchMethod,
	}, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// BulkEnrichmentResult contains the summary of a bulk enrichment operation.
type BulkEnrichmentResult struct {
	TotalBooks int      `json:"total_books"`
	Enriched   int      `json:"enriched"`
	Failed     int      `json:"failed"`
	Skipped    int      `json:"skipped"`
	Errors     []string `json:"errors,omitempty"`
}

// EnrichAllMissing enriches all books that are missing metadata (cover, publisher, or year).
func (e *Enricher) EnrichAllMissing(ctx context.Context) (*BulkEnrichmentResult, error) {
	// Check if a sync is already running (and isn't stale)
	if e.progressReporter != nil {
		running, err := e.progressReporter.IsSyncRunning()
		if err != nil {
			return nil, fmt.Errorf("check sync status: %w", err)
		}
		if running {
			return nil, fmt.Errorf("metadata sync is already in progress")
		}
	}

	books, err := e.db.GetBooksMissingMetadata()
	if err != nil {
		return nil, fmt.Errorf("get books missing metadata: %w", err)
	}

	result := &BulkEnrichmentResult{
		TotalBooks: len(books),
	}

	// Start progress tracking
	if e.progressReporter != nil {
		if err := e.progressReporter.StartSync(len(books)); err != nil {
			return nil, fmt.Errorf("start sync progress: %w", err)
		}
	}

	for i, book := range books {
		select {
		case <-ctx.Done():
			result.Errors = append(result.Errors, "operation cancelled")
			if e.progressReporter != nil {
				_ = e.progressReporter.CompleteSync(false, "operation cancelled")
			}
			return result, ctx.Err()
		default:
		}

		// Update progress with current book
		if e.progressReporter != nil {
			_ = e.progressReporter.UpdateProgress(
				i,
				result.Enriched,
				result.Failed,
				result.Skipped,
				book.Title,
			)
		}

		enrichResult, err := e.EnrichBook(ctx, book.ID)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", book.Title, err))
			continue
		}

		if len(enrichResult.FieldsUpdated) > 0 {
			result.Enriched++
		} else {
			result.Skipped++
		}
	}

	// Mark sync as complete
	if e.progressReporter != nil {
		errorMsg := ""
		if len(result.Errors) > 0 {
			errorMsg = fmt.Sprintf("%d errors occurred", len(result.Errors))
		}
		_ = e.progressReporter.CompleteSync(result.Failed == 0, errorMsg)
	}

	return result, nil
}

// buildUpdates compares existing book data with fetched metadata and returns
// only the fields that should be updated (empty fields or refreshed data).
func (e *Enricher) buildUpdates(book *entities.Book, metadata *BookMetadata) (BookUpdateFields, []string) {
	var updates BookUpdateFields
	var fieldsUpdated []string

	// Update ISBN if we found one and book doesn't have one
	if book.ISBN == "" && metadata.ISBN != "" {
		updates.ISBN = &metadata.ISBN
		fieldsUpdated = append(fieldsUpdated, "isbn")
	}

	// Update cover URL if empty or if we have a better one
	if metadata.CoverURL != "" && (book.CoverURL == "" || book.CoverURL != metadata.CoverURL) {
		updates.CoverURL = &metadata.CoverURL
		fieldsUpdated = append(fieldsUpdated, "cover_url")
	}

	// Update publisher if empty
	if book.Publisher == "" && metadata.Publisher != "" {
		updates.Publisher = &metadata.Publisher
		fieldsUpdated = append(fieldsUpdated, "publisher")
	}

	// Update publication year if not set
	if book.PublicationYear == 0 && metadata.PublicationYear > 0 {
		updates.PublicationYear = &metadata.PublicationYear
		fieldsUpdated = append(fieldsUpdated, "publication_year")
	}

	return updates, fieldsUpdated
}
