package database

import "github.com/mrlokans/assistant/internal/entities"

// SetHighlightFavourite updates the favourite status of a highlight.
func (d *Database) SetHighlightFavourite(highlightID uint, isFavourite bool) error {
	return d.DB.Model(&entities.Highlight{}).
		Where("id = ?", highlightID).
		Update("is_favorite", isFavourite).Error
}

// GetFavouriteHighlights returns all favourite highlights for a user with pagination.
// Returns the highlights, total count, and any error.
func (d *Database) GetFavouriteHighlights(userID uint, limit, offset int) ([]entities.Highlight, int64, error) {
	var highlights []entities.Highlight
	var total int64

	query := d.DB.Model(&entities.Highlight{}).Where("is_favorite = ?", true)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = d.DB.Preload("Tags").Preload("Book").Preload("Source").
		Where("is_favorite = ?", true).
		Order("updated_at DESC")

	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&highlights).Error
	return highlights, total, err
}

// GetFavouriteHighlightsByBook returns all favourite highlights for a specific book.
func (d *Database) GetFavouriteHighlightsByBook(bookID uint) ([]entities.Highlight, error) {
	var highlights []entities.Highlight
	err := d.DB.Preload("Tags").
		Where("book_id = ? AND is_favorite = ?", bookID, true).
		Order("location_value ASC, highlighted_at ASC").
		Find(&highlights).Error
	return highlights, err
}

// GetFavouriteCount returns the total number of favourite highlights.
func (d *Database) GetFavouriteCount(userID uint) (int64, error) {
	var count int64
	query := d.DB.Model(&entities.Highlight{}).Where("is_favorite = ?", true)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Count(&count).Error
	return count, err
}
