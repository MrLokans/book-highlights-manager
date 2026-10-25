// Package books provides database operations for book and highlight management.
//
// # Usage
//
//	repo := books.NewRepository(db)
//	book, err := repo.GetBookByID(123)
package books

import (
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/mrlokans/assistant/internal/database/sources"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all book and highlight database operations.
type Repository struct {
	db      *gorm.DB
	sources *sources.Repository
}

// NewRepository creates a new books repository.
func NewRepository(db *gorm.DB, sources *sources.Repository) *Repository {
	return &Repository{db: db, sources: sources}
}

// GetBookByID retrieves a book by its ID with all related data.
func (r *Repository) GetBookByID(id uint) (*entities.Book, error) {
	var book entities.Book
	err := r.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").First(&book, id).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

// GetBookByTitleAndAuthor retrieves a book by title and author.
func (r *Repository) GetBookByTitleAndAuthor(title, author string) (*entities.Book, error) {
	var book entities.Book
	err := r.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Source").Where("title = ? AND author = ?", title, author).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

// GetAllBooks retrieves all books with their highlights.
func (r *Repository) GetAllBooks() ([]entities.Book, error) {
	var books []entities.Book
	err := r.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").Find(&books).Error
	return books, err
}

// SearchBooks searches books by title or author (case-insensitive partial match).
func (r *Repository) SearchBooks(query string) ([]entities.Book, error) {
	var books []entities.Book
	searchPattern := "%" + query + "%"
	err := r.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").
		Where("LOWER(title) LIKE LOWER(?) OR LOWER(author) LIKE LOWER(?)", searchPattern, searchPattern).
		Find(&books).Error
	return books, err
}

// SaveBook upserts a book and its highlights, deduplicating by text + location + timestamp.
// Skips books and highlights that have been permanently deleted.
func (r *Repository) SaveBook(book *entities.Book) error {
	// Check if this book was permanently deleted
	deleted, err := r.IsBookDeleted(book.Title, book.Author)
	if err != nil {
		return fmt.Errorf("failed to check if book was deleted: %w", err)
	}
	if deleted {
		slog.Info("Skipping book: permanently deleted", "title", book.Title, "author", book.Author)
		return nil
	}

	originalSource := r.resolveBookSource(book)
	r.filterHighlights(book)

	// Check if book already exists
	var existingBook entities.Book
	result := r.db.Preload("Highlights").Where("title = ? AND author = ?", book.Title, book.Author).First(&existingBook)

	var saveErr error
	switch result.Error {
	case nil:
		// Book exists, merge highlights
		book.ID = existingBook.ID

		type existingHighlightInfo struct {
			ID         uint
			IsFavorite bool
		}
		existingHighlights := make(map[string]existingHighlightInfo)
		for i := range existingBook.Highlights {
			h := &existingBook.Highlights[i]
			key := highlightKey(h)
			existingHighlights[key] = existingHighlightInfo{ID: h.ID, IsFavorite: h.IsFavorite}
		}

		var newHighlights []entities.Highlight
		for _, h := range book.Highlights {
			key := highlightKey(&h)
			if existing, exists := existingHighlights[key]; exists {
				h.ID = existing.ID
				h.IsFavorite = existing.IsFavorite
			}
			h.BookID = book.ID
			newHighlights = append(newHighlights, h)
		}
		book.Highlights = newHighlights

		saveErr = r.db.Transaction(func(tx *gorm.DB) error {
			// Updates skips zero fields, so metadata the import doesn't carry (cover, ISBN, created_at) survives.
			if err := tx.Model(&existingBook).Omit(clause.Associations).Updates(book).Error; err != nil {
				return err
			}
			// Save ignores CreateBatchSize; chunk to stay under SQLite's bound-parameter limit.
			for chunk := range slices.Chunk(book.Highlights, 500) {
				if err := tx.Omit("Source").Save(&chunk).Error; err != nil {
					return err
				}
			}
			return nil
		})
	case gorm.ErrRecordNotFound:
		saveErr = r.db.Omit("Source", "Highlights.Source").Create(book).Error
	default:
		saveErr = result.Error
	}

	book.Source = originalSource
	return saveErr
}

func (r *Repository) resolveBookSource(book *entities.Book) entities.Source {
	originalSource := book.Source
	if book.SourceID == 0 && book.Source.Name != "" {
		source, err := r.sources.GetByName(book.Source.Name)
		if err == nil && source != nil {
			book.SourceID = source.ID
			originalSource = *source
		}
	}
	return originalSource
}

// filterHighlights resolves highlight sources and drops permanently deleted highlights,
// using one source lookup per distinct name and batched deleted-key queries.
func (r *Repository) filterHighlights(book *entities.Book) {
	if len(book.Highlights) == 0 {
		return
	}

	keys := make([]string, len(book.Highlights))
	for i := range book.Highlights {
		keys[i] = highlightKey(&book.Highlights[i])
	}
	deleted := r.deletedHighlightKeys(keys)

	sourceIDs := make(map[string]uint)
	var filtered []entities.Highlight
	for i := range book.Highlights {
		h := &book.Highlights[i]
		if deleted[keys[i]] {
			continue
		}
		if h.SourceID == 0 && h.Source.Name != "" {
			id, ok := sourceIDs[h.Source.Name]
			if !ok {
				if source, err := r.sources.GetByName(h.Source.Name); err == nil && source != nil {
					id = source.ID
				}
				sourceIDs[h.Source.Name] = id
			}
			h.SourceID = id
		}
		filtered = append(filtered, *h)
	}
	book.Highlights = filtered
}

// deletedHighlightKeys returns which of keys were permanently deleted.
// On query errors it logs and treats the highlights as not deleted.
func (r *Repository) deletedHighlightKeys(keys []string) map[string]bool {
	deleted := make(map[string]bool)
	for chunk := range slices.Chunk(keys, 500) { // stay under SQLite's bound-parameter limit
		var found []string
		if err := r.db.Model(&entities.DeletedEntity{}).
			Where("entity_type = ? AND entity_key IN ?", "highlight", chunk).
			Pluck("entity_key", &found).Error; err != nil {
			slog.Warn("Failed to load deleted highlights; importing them", "error", err)
		}
		for _, k := range found {
			deleted[k] = true
		}
	}
	return deleted
}

// DeleteBook performs a soft delete.
func (r *Repository) DeleteBook(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("book_id = ?", id).Delete(&entities.Highlight{}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM book_tags WHERE book_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&entities.Book{}, id).Error
	})
}

// DeleteBookPermanently hard deletes a book and records deletion to prevent re-import.
func (r *Repository) DeleteBookPermanently(id uint) error {
	var book entities.Book
	if err := r.db.Unscoped().First(&book, id).Error; err != nil {
		return err
	}

	entityKey := fmt.Sprintf("%s|%s", book.Title, book.Author)

	return r.db.Transaction(func(tx *gorm.DB) error {
		var highlightIDs []uint
		tx.Model(&entities.Highlight{}).Unscoped().Where("book_id = ?", id).Pluck("id", &highlightIDs)

		if len(highlightIDs) > 0 {
			if err := tx.Exec("DELETE FROM highlight_tags WHERE highlight_id IN ?", highlightIDs).Error; err != nil {
				return err
			}
		}

		if err := tx.Unscoped().Where("book_id = ?", id).Delete(&entities.Highlight{}).Error; err != nil {
			return err
		}

		if err := tx.Exec("DELETE FROM book_tags WHERE book_id = ?", id).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Delete(&entities.Book{}, id).Error; err != nil {
			return err
		}

		deletedEntity := entities.DeletedEntity{
			EntityType: "book",
			EntityKey:  entityKey,
			SourceID:   book.SourceID,
			DeletedAt:  time.Now(),
		}
		return tx.Create(&deletedEntity).Error
	})
}

// UpdateBookMetadata updates specific metadata fields.
func (r *Repository) UpdateBookMetadata(id uint, fields map[string]any) error {
	return r.db.Model(&entities.Book{}).Where("id = ?", id).Updates(fields).Error
}

// GetBooksMissingMetadata returns books without cover URL, publisher, or publication year.
func (r *Repository) GetBooksMissingMetadata() ([]entities.Book, error) {
	var books []entities.Book
	err := r.db.Where(
		"cover_url = '' OR cover_url IS NULL OR publisher = '' OR publisher IS NULL OR publication_year = 0 OR publication_year IS NULL",
	).Find(&books).Error
	return books, err
}

// IsBookDeleted checks if a book was permanently deleted.
func (r *Repository) IsBookDeleted(title, author string) (bool, error) {
	entityKey := fmt.Sprintf("%s|%s", title, author)
	var count int64
	err := r.db.Model(&entities.DeletedEntity{}).
		Where("entity_type = ? AND entity_key = ?", "book", entityKey).
		Count(&count).Error
	return count > 0, err
}

// GetStats returns total book and highlight counts.
func (r *Repository) GetStats() (totalBooks int64, totalHighlights int64, err error) {
	err = r.db.Model(&entities.Book{}).Count(&totalBooks).Error
	if err != nil {
		return
	}
	err = r.db.Model(&entities.Highlight{}).Count(&totalHighlights).Error
	return
}
