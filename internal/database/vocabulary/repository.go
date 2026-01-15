// Package vocabulary provides database operations for vocabulary word management.
//
// This package implements the VocabularyStore interface defined in internal/http/vocabulary.go.
//
// # Interface Implementation
//
//	var _ http.VocabularyStore = (*Repository)(nil)
//
// # Usage
//
//	repo := vocabulary.NewRepository(db)
//	words, total, err := repo.GetAllWords(userID, 20, 0)
package vocabulary

import (
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all vocabulary database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new vocabulary repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// AddWord creates a new vocabulary word entry.
func (r *Repository) AddWord(word *entities.Word) error {
	return r.db.Create(word).Error
}

// GetAllWords returns all words for a user with pagination.
func (r *Repository) GetAllWords(userID uint, limit, offset int) ([]entities.Word, int64, error) {
	var words []entities.Word
	var total int64

	query := r.db.Model(&entities.Word{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = r.db.Preload("Definitions").Preload("Book").Preload("Highlight")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	query = query.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&words).Error
	return words, total, err
}

// GetWordByID retrieves a word by ID with all relationships.
func (r *Repository) GetWordByID(id uint) (*entities.Word, error) {
	var word entities.Word
	err := r.db.Preload("Definitions").Preload("Book").Preload("Highlight").First(&word, id).Error
	if err != nil {
		return nil, err
	}
	return &word, nil
}

// UpdateWord updates a word's fields.
func (r *Repository) UpdateWord(word *entities.Word) error {
	return r.db.Save(word).Error
}

// DeleteWord removes a word and its definitions.
func (r *Repository) DeleteWord(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("word_id = ?", id).Delete(&entities.WordDefinition{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entities.Word{}, id).Error
	})
}

// GetPendingWords returns words awaiting enrichment.
func (r *Repository) GetPendingWords(limit int) ([]entities.Word, error) {
	var words []entities.Word
	query := r.db.Where("status = ?", entities.WordStatusPending).Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&words).Error
	return words, err
}

// SaveDefinitions saves definitions for a word, replacing existing ones.
func (r *Repository) SaveDefinitions(wordID uint, definitions []entities.WordDefinition) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("word_id = ?", wordID).Delete(&entities.WordDefinition{}).Error; err != nil {
			return err
		}

		for i := range definitions {
			definitions[i].WordID = wordID
			definitions[i].ID = 0
			if err := tx.Create(&definitions[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateWordStatus updates the enrichment status of a word.
func (r *Repository) UpdateWordStatus(id uint, status entities.WordStatus, errorMsg string) error {
	updates := map[string]any{
		"status": status,
	}
	if errorMsg != "" {
		updates["enrichment_error"] = errorMsg
	} else {
		updates["enrichment_error"] = ""
	}
	return r.db.Model(&entities.Word{}).Where("id = ?", id).Updates(updates).Error
}

// GetWordsByHighlight returns all words for a specific highlight.
func (r *Repository) GetWordsByHighlight(highlightID uint) ([]entities.Word, error) {
	var words []entities.Word
	err := r.db.Preload("Definitions").Where("highlight_id = ?", highlightID).Find(&words).Error
	return words, err
}

// GetWordsByBook returns all words for a specific book.
func (r *Repository) GetWordsByBook(bookID uint) ([]entities.Word, error) {
	var words []entities.Word
	err := r.db.Preload("Definitions").Where("book_id = ?", bookID).Find(&words).Error
	return words, err
}

// FindWordBySource checks if a word already exists from the same source.
func (r *Repository) FindWordBySource(word, sourceBookTitle, sourceBookAuthor, sourceHighlightText string, userID uint) (*entities.Word, error) {
	var existing entities.Word
	query := r.db.Where("word = ? AND source_book_title = ? AND source_book_author = ? AND source_highlight_text = ?",
		word, sourceBookTitle, sourceBookAuthor, sourceHighlightText)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.First(&existing).Error
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

// SearchWords searches for words by word text.
func (r *Repository) SearchWords(query string, userID uint, limit int) ([]entities.Word, error) {
	var words []entities.Word
	searchPattern := "%" + query + "%"
	q := r.db.Preload("Definitions").Where("LOWER(word) LIKE LOWER(?)", searchPattern)
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&words).Error
	return words, err
}

// GetVocabularyStats returns vocabulary statistics.
func (r *Repository) GetVocabularyStats(userID uint) (total, pending, enriched, failed int64, err error) {
	baseQuery := r.db.Model(&entities.Word{})
	if userID > 0 {
		baseQuery = baseQuery.Where("user_id = ?", userID)
	}

	if err = baseQuery.Count(&total).Error; err != nil {
		return
	}

	pendingQuery := r.db.Model(&entities.Word{}).Where("status = ?", entities.WordStatusPending)
	if userID > 0 {
		pendingQuery = pendingQuery.Where("user_id = ?", userID)
	}
	if err = pendingQuery.Count(&pending).Error; err != nil {
		return
	}

	enrichedQuery := r.db.Model(&entities.Word{}).Where("status = ?", entities.WordStatusEnriched)
	if userID > 0 {
		enrichedQuery = enrichedQuery.Where("user_id = ?", userID)
	}
	if err = enrichedQuery.Count(&enriched).Error; err != nil {
		return
	}

	failedQuery := r.db.Model(&entities.Word{}).Where("status = ?", entities.WordStatusFailed)
	if userID > 0 {
		failedQuery = failedQuery.Where("user_id = ?", userID)
	}
	err = failedQuery.Count(&failed).Error
	return
}

// GetWordsByStatus returns words filtered by status with pagination.
func (r *Repository) GetWordsByStatus(userID uint, status entities.WordStatus, limit, offset int) ([]entities.Word, int64, error) {
	var words []entities.Word
	var total int64

	query := r.db.Model(&entities.Word{}).Where("status = ?", status)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = r.db.Preload("Definitions").Preload("Book").Preload("Highlight").
		Where("status = ?", status)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	query = query.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&words).Error
	return words, total, err
}

// GetHighlightByID retrieves a highlight by ID (for VocabularyStore interface).
func (r *Repository) GetHighlightByID(id uint) (*entities.Highlight, error) {
	var highlight entities.Highlight
	err := r.db.Preload("Tags").Preload("Source").First(&highlight, id).Error
	if err != nil {
		return nil, err
	}
	return &highlight, nil
}

// GetBookByID retrieves a book by ID (for VocabularyStore interface).
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
