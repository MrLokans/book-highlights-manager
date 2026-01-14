package database

import (
	"github.com/mrlokans/assistant/internal/entities"
	"gorm.io/gorm"
)

// AddWord creates a new vocabulary word entry.
func (d *Database) AddWord(word *entities.Word) error {
	return d.DB.Create(word).Error
}

// GetAllWords returns all words for a user with pagination.
func (d *Database) GetAllWords(userID uint, limit, offset int) ([]entities.Word, int64, error) {
	var words []entities.Word
	var total int64

	query := d.DB.Model(&entities.Word{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = d.DB.Preload("Definitions").Preload("Book").Preload("Highlight")
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
func (d *Database) GetWordByID(id uint) (*entities.Word, error) {
	var word entities.Word
	err := d.DB.Preload("Definitions").Preload("Book").Preload("Highlight").First(&word, id).Error
	if err != nil {
		return nil, err
	}
	return &word, nil
}

// UpdateWord updates a word's fields.
func (d *Database) UpdateWord(word *entities.Word) error {
	return d.DB.Save(word).Error
}

// DeleteWord removes a word and its definitions.
func (d *Database) DeleteWord(id uint) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("word_id = ?", id).Delete(&entities.WordDefinition{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entities.Word{}, id).Error
	})
}

// GetPendingWords returns words awaiting enrichment.
func (d *Database) GetPendingWords(limit int) ([]entities.Word, error) {
	var words []entities.Word
	query := d.DB.Where("status = ?", entities.WordStatusPending).Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&words).Error
	return words, err
}

// SaveDefinitions saves definitions for a word, replacing any existing ones.
func (d *Database) SaveDefinitions(wordID uint, definitions []entities.WordDefinition) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("word_id = ?", wordID).Delete(&entities.WordDefinition{}).Error; err != nil {
			return err
		}

		for i := range definitions {
			definitions[i].WordID = wordID
			definitions[i].ID = 0 // Reset ID for new insert
			if err := tx.Create(&definitions[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateWordStatus updates the enrichment status of a word.
func (d *Database) UpdateWordStatus(id uint, status entities.WordStatus, errorMsg string) error {
	updates := map[string]any{
		"status": status,
	}
	if errorMsg != "" {
		updates["enrichment_error"] = errorMsg
	} else {
		updates["enrichment_error"] = ""
	}
	return d.DB.Model(&entities.Word{}).Where("id = ?", id).Updates(updates).Error
}

// GetWordsByHighlight returns all words for a specific highlight.
func (d *Database) GetWordsByHighlight(highlightID uint) ([]entities.Word, error) {
	var words []entities.Word
	err := d.DB.Preload("Definitions").Where("highlight_id = ?", highlightID).Find(&words).Error
	return words, err
}

// GetWordsByBook returns all words for a specific book.
func (d *Database) GetWordsByBook(bookID uint) ([]entities.Word, error) {
	var words []entities.Word
	err := d.DB.Preload("Definitions").Where("book_id = ?", bookID).Find(&words).Error
	return words, err
}

// FindWordBySource checks if a word already exists from the same source.
// Used for re-import deduplication.
func (d *Database) FindWordBySource(word, sourceBookTitle, sourceBookAuthor, sourceHighlightText string, userID uint) (*entities.Word, error) {
	var existing entities.Word
	query := d.DB.Where("word = ? AND source_book_title = ? AND source_book_author = ? AND source_highlight_text = ?",
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
func (d *Database) SearchWords(query string, userID uint, limit int) ([]entities.Word, error) {
	var words []entities.Word
	searchPattern := "%" + query + "%"
	q := d.DB.Preload("Definitions").Where("LOWER(word) LIKE LOWER(?)", searchPattern)
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
func (d *Database) GetVocabularyStats(userID uint) (total, pending, enriched, failed int64, err error) {
	baseQuery := d.DB.Model(&entities.Word{})
	if userID > 0 {
		baseQuery = baseQuery.Where("user_id = ?", userID)
	}

	if err = baseQuery.Count(&total).Error; err != nil {
		return
	}

	pendingQuery := d.DB.Model(&entities.Word{}).Where("status = ?", entities.WordStatusPending)
	if userID > 0 {
		pendingQuery = pendingQuery.Where("user_id = ?", userID)
	}
	if err = pendingQuery.Count(&pending).Error; err != nil {
		return
	}

	enrichedQuery := d.DB.Model(&entities.Word{}).Where("status = ?", entities.WordStatusEnriched)
	if userID > 0 {
		enrichedQuery = enrichedQuery.Where("user_id = ?", userID)
	}
	if err = enrichedQuery.Count(&enriched).Error; err != nil {
		return
	}

	failedQuery := d.DB.Model(&entities.Word{}).Where("status = ?", entities.WordStatusFailed)
	if userID > 0 {
		failedQuery = failedQuery.Where("user_id = ?", userID)
	}
	err = failedQuery.Count(&failed).Error
	return
}

// GetWordsByStatus returns words filtered by status with pagination.
func (d *Database) GetWordsByStatus(userID uint, status entities.WordStatus, limit, offset int) ([]entities.Word, int64, error) {
	var words []entities.Word
	var total int64

	query := d.DB.Model(&entities.Word{}).Where("status = ?", status)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = d.DB.Preload("Definitions").Preload("Book").Preload("Highlight").
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
