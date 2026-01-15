// Package books provides database operations for book and highlight management.
//
// This package implements the BookReader and BookExporter interfaces defined in
// internal/services/interfaces.go.
//
// # Interface Implementation
//
//	var _ services.BookReader = (*Repository)(nil)
//
// # Usage
//
//	repo := books.NewRepository(db)
//	book, err := repo.GetBookByID(123)
package books

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all book and highlight database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new books repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
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

// GetBookByTitleAndAuthorForUser retrieves a book by title, author, and user.
func (r *Repository) GetBookByTitleAndAuthorForUser(title, author string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := r.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Source").
		Where("title = ? AND author = ? AND user_id = ?", title, author, userID).
		First(&book).Error
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

// GetAllBooksForUser retrieves all books for a specific user.
func (r *Repository) GetAllBooksForUser(userID uint) ([]entities.Book, error) {
	var books []entities.Book
	err := r.db.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").Where("user_id = ?", userID).Find(&books).Error
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
func (r *Repository) SaveBook(book *entities.Book, getSourceByName func(string) (*entities.Source, error), isBookDeleted func(string, string, uint) (bool, error), isHighlightDeleted func(string, int, time.Time, uint) (bool, error)) error {
	// Check if this book was permanently deleted
	deleted, err := isBookDeleted(book.Title, book.Author, book.UserID)
	if err != nil {
		return fmt.Errorf("failed to check if book was deleted: %w", err)
	}
	if deleted {
		log.Printf("Skipping book '%s' by %s: permanently deleted", book.Title, book.Author)
		return nil
	}

	// If Source.Name is set but SourceID is 0, look up the source
	originalSource := book.Source
	if book.SourceID == 0 && book.Source.Name != "" {
		source, err := getSourceByName(book.Source.Name)
		if err == nil && source != nil {
			book.SourceID = source.ID
			originalSource = *source
		}
	}

	// Also fix SourceID for all highlights and filter out deleted ones
	var filteredHighlights []entities.Highlight
	for i := range book.Highlights {
		if book.Highlights[i].SourceID == 0 && book.Highlights[i].Source.Name != "" {
			source, err := getSourceByName(book.Highlights[i].Source.Name)
			if err == nil && source != nil {
				book.Highlights[i].SourceID = source.ID
			}
		}

		h := &book.Highlights[i]
		highlightDeleted, _ := isHighlightDeleted(h.Text, h.LocationValue, h.HighlightedAt, book.UserID)
		if !highlightDeleted {
			filteredHighlights = append(filteredHighlights, *h)
		}
	}
	book.Highlights = filteredHighlights

	// Check if book already exists
	var existingBook entities.Book
	result := r.db.Preload("Highlights").Where("title = ? AND author = ? AND user_id = ?", book.Title, book.Author, book.UserID).First(&existingBook)

	var saveErr error
	if result.Error == nil {
		// Book exists, merge highlights
		book.ID = existingBook.ID

		type existingHighlightInfo struct {
			ID         uint
			IsFavorite bool
		}
		existingHighlights := make(map[string]existingHighlightInfo)
		for _, h := range existingBook.Highlights {
			key := fmt.Sprintf("%s|%d|%s", h.Text, h.LocationValue, h.HighlightedAt.Format("2006-01-02 15:04:05"))
			existingHighlights[key] = existingHighlightInfo{ID: h.ID, IsFavorite: h.IsFavorite}
		}

		var newHighlights []entities.Highlight
		for _, h := range book.Highlights {
			key := fmt.Sprintf("%s|%d|%s", h.Text, h.LocationValue, h.HighlightedAt.Format("2006-01-02 15:04:05"))
			if existing, exists := existingHighlights[key]; exists {
				h.ID = existing.ID
				h.IsFavorite = existing.IsFavorite
			}
			h.BookID = book.ID
			newHighlights = append(newHighlights, h)
		}
		book.Highlights = newHighlights

		saveErr = r.db.Session(&gorm.Session{FullSaveAssociations: true}).Omit("Source", "Highlights.Source").Save(book).Error
	} else if result.Error == gorm.ErrRecordNotFound {
		saveErr = r.db.Omit("Source", "Highlights.Source").Create(book).Error
	} else {
		saveErr = result.Error
	}

	book.Source = originalSource
	return saveErr
}

// SaveBookForUser saves a book for a specific user.
func (r *Repository) SaveBookForUser(book *entities.Book, userID uint, getSourceByName func(string) (*entities.Source, error), isBookDeleted func(string, string, uint) (bool, error), isHighlightDeleted func(string, int, time.Time, uint) (bool, error)) error {
	book.UserID = userID
	return r.SaveBook(book, getSourceByName, isBookDeleted, isHighlightDeleted)
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
func (r *Repository) DeleteBookPermanently(id uint, userID uint) error {
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
			UserID:     userID,
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

// FindBookByISBN finds a book by ISBN for a user.
func (r *Repository) FindBookByISBN(isbn string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := r.db.Where("isbn = ? AND user_id = ?", isbn, userID).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

// FindBookByFileHash finds a book by file hash for a user.
func (r *Repository) FindBookByFileHash(hash string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := r.db.Where("file_hash = ? AND user_id = ?", hash, userID).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

// IsBookDeleted checks if a book was permanently deleted.
func (r *Repository) IsBookDeleted(title, author string, userID uint) (bool, error) {
	entityKey := fmt.Sprintf("%s|%s", title, author)
	var count int64
	err := r.db.Model(&entities.DeletedEntity{}).
		Where("entity_type = ? AND entity_key = ? AND (user_id = ? OR user_id = 0)", "book", entityKey, userID).
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

// GetStatsForUser returns book and highlight counts for a user.
func (r *Repository) GetStatsForUser(userID uint) (totalBooks int64, totalHighlights int64, err error) {
	err = r.db.Model(&entities.Book{}).Where("user_id = ?", userID).Count(&totalBooks).Error
	if err != nil {
		return
	}
	err = r.db.Model(&entities.Highlight{}).Where("user_id = ?", userID).Count(&totalHighlights).Error
	return
}
