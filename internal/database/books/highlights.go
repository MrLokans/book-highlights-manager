package books

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// GetHighlightByID retrieves a highlight by ID with related data.
func (r *Repository) GetHighlightByID(id uint) (*entities.Highlight, error) {
	var highlight entities.Highlight
	err := r.db.Preload("Tags").Preload("Source").First(&highlight, id).Error
	if err != nil {
		return nil, err
	}
	return &highlight, nil
}

// GetHighlightsForBook retrieves all highlights for a book.
func (r *Repository) GetHighlightsForBook(bookID uint) ([]entities.Highlight, error) {
	var highlights []entities.Highlight
	err := r.db.Preload("Tags").Where("book_id = ?", bookID).
		Order("location_value ASC, highlighted_at ASC").Find(&highlights).Error
	return highlights, err
}

// GetHighlightsForUser retrieves highlights for a user with pagination.
func (r *Repository) GetHighlightsForUser(userID uint, limit, offset int) ([]entities.Highlight, error) {
	var highlights []entities.Highlight
	query := r.db.Preload("Tags").Preload("Source").Where("user_id = ?", userID).Order("highlighted_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&highlights).Error
	return highlights, err
}

// UpdateHighlight updates a highlight.
func (r *Repository) UpdateHighlight(highlight *entities.Highlight) error {
	return r.db.Save(highlight).Error
}

// DeleteHighlight performs a soft delete.
func (r *Repository) DeleteHighlight(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM highlight_tags WHERE highlight_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&entities.Highlight{}, id).Error
	})
}

// DeleteHighlightPermanently hard deletes a highlight and records deletion.
func (r *Repository) DeleteHighlightPermanently(id uint, userID uint) error {
	var highlight entities.Highlight
	if err := r.db.Unscoped().First(&highlight, id).Error; err != nil {
		return err
	}

	entityKey := fmt.Sprintf("%s|%d|%s", highlight.Text, highlight.LocationValue, highlight.HighlightedAt.Format("2006-01-02 15:04:05"))

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM highlight_tags WHERE highlight_id = ?", id).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Delete(&entities.Highlight{}, id).Error; err != nil {
			return err
		}

		deletedEntity := entities.DeletedEntity{
			UserID:     userID,
			EntityType: "highlight",
			EntityKey:  entityKey,
			SourceID:   highlight.SourceID,
			DeletedAt:  time.Now(),
		}
		return tx.Create(&deletedEntity).Error
	})
}

// IsHighlightDeleted checks if a highlight was permanently deleted.
func (r *Repository) IsHighlightDeleted(text string, locationValue int, highlightedAt time.Time, userID uint) (bool, error) {
	entityKey := fmt.Sprintf("%s|%d|%s", text, locationValue, highlightedAt.Format("2006-01-02 15:04:05"))
	var count int64
	err := r.db.Model(&entities.DeletedEntity{}).
		Where("entity_type = ? AND entity_key = ? AND (user_id = ? OR user_id = 0)", "highlight", entityKey, userID).
		Count(&count).Error
	return count > 0, err
}
