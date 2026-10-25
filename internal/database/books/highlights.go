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
func (r *Repository) DeleteHighlightPermanently(id uint) error {
	var highlight entities.Highlight
	if err := r.db.Unscoped().First(&highlight, id).Error; err != nil {
		return err
	}

	entityKey := highlightKey(&highlight)

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM highlight_tags WHERE highlight_id = ?", id).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Delete(&entities.Highlight{}, id).Error; err != nil {
			return err
		}

		deletedEntity := entities.DeletedEntity{
			EntityType: "highlight",
			EntityKey:  entityKey,
			SourceID:   highlight.SourceID,
			DeletedAt:  time.Now(),
		}
		return tx.Create(&deletedEntity).Error
	})
}

// highlightKey identifies a highlight across re-imports; also the DeletedEntity key.
func highlightKey(h *entities.Highlight) string {
	return fmt.Sprintf("%s|%d|%s", h.Text, h.LocationValue, h.HighlightedAt.Format("2006-01-02 15:04:05"))
}

// IsHighlightDeleted checks if a highlight was permanently deleted.
func (r *Repository) IsHighlightDeleted(text string, locationValue int, highlightedAt time.Time) (bool, error) {
	entityKey := highlightKey(&entities.Highlight{Text: text, LocationValue: locationValue, HighlightedAt: highlightedAt})
	var count int64
	err := r.db.Model(&entities.DeletedEntity{}).
		Where("entity_type = ? AND entity_key = ?", "highlight", entityKey).
		Count(&count).Error
	return count > 0, err
}
