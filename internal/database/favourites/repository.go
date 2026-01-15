// Package favourites provides database operations for favourite highlight management.
//
// This package implements the FavouritesStore interface defined in internal/http/favourites.go.
//
// # Interface Implementation
//
//	var _ http.FavouritesStore = (*Repository)(nil)
//
// # Usage
//
//	repo := favourites.NewRepository(db)
//	highlights, total, err := repo.GetFavouriteHighlights(userID, 20, 0)
package favourites

import (
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all favourites database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new favourites repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// SetHighlightFavourite updates the favourite status of a highlight.
func (r *Repository) SetHighlightFavourite(highlightID uint, isFavourite bool) error {
	return r.db.Model(&entities.Highlight{}).
		Where("id = ?", highlightID).
		Update("is_favorite", isFavourite).Error
}

// GetFavouriteHighlights returns all favourite highlights for a user with pagination.
func (r *Repository) GetFavouriteHighlights(userID uint, limit, offset int) ([]entities.Highlight, int64, error) {
	var highlights []entities.Highlight
	var total int64

	query := r.db.Model(&entities.Highlight{}).Where("is_favorite = ?", true)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = r.db.Preload("Tags").Preload("Book").Preload("Source").
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
func (r *Repository) GetFavouriteHighlightsByBook(bookID uint) ([]entities.Highlight, error) {
	var highlights []entities.Highlight
	err := r.db.Preload("Tags").
		Where("book_id = ? AND is_favorite = ?", bookID, true).
		Order("location_value ASC, highlighted_at ASC").
		Find(&highlights).Error
	return highlights, err
}

// GetFavouriteCount returns the total number of favourite highlights.
func (r *Repository) GetFavouriteCount(userID uint) (int64, error) {
	var count int64
	query := r.db.Model(&entities.Highlight{}).Where("is_favorite = ?", true)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Count(&count).Error
	return count, err
}

// GetHighlightByID retrieves a highlight by ID (for FavouritesStore interface).
func (r *Repository) GetHighlightByID(id uint) (*entities.Highlight, error) {
	var highlight entities.Highlight
	err := r.db.Preload("Tags").Preload("Source").First(&highlight, id).Error
	if err != nil {
		return nil, err
	}
	return &highlight, nil
}
