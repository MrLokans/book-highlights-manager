// Package tags provides database operations for tag management.
//
// This package implements the TagStore interface defined in internal/http/tags.go.
//
// # Interface Implementation
//
//	var _ http.TagStore = (*Repository)(nil)
//
// # Usage
//
//	repo := tags.NewRepository(db)
//	tag, err := repo.GetOrCreateTag("fiction", userID)
package tags

import (
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all tag database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new tags repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateTag creates a new tag.
func (r *Repository) CreateTag(name string, userID uint) (*entities.Tag, error) {
	tag := &entities.Tag{
		Name:   name,
		UserID: userID,
	}
	if err := r.db.Create(tag).Error; err != nil {
		return nil, err
	}
	return tag, nil
}

// GetOrCreateTag retrieves or creates a tag (case-insensitive).
func (r *Repository) GetOrCreateTag(name string, userID uint) (*entities.Tag, error) {
	var tag entities.Tag
	err := r.db.Where("LOWER(name) = LOWER(?) AND user_id = ?", name, userID).First(&tag).Error
	if err == gorm.ErrRecordNotFound {
		return r.CreateTag(name, userID)
	}
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// GetTagsForUser retrieves all tags for a user.
func (r *Repository) GetTagsForUser(userID uint) ([]entities.Tag, error) {
	var tags []entities.Tag
	err := r.db.Where("user_id = ?", userID).Find(&tags).Error
	return tags, err
}

// SearchTags searches tags by name (case-insensitive partial match).
func (r *Repository) SearchTags(query string, userID uint) ([]entities.Tag, error) {
	var tags []entities.Tag
	searchPattern := "%" + query + "%"
	err := r.db.Where("user_id = ? AND LOWER(name) LIKE LOWER(?)", userID, searchPattern).Find(&tags).Error
	return tags, err
}

// GetTagByID retrieves a tag by ID.
func (r *Repository) GetTagByID(id uint) (*entities.Tag, error) {
	var tag entities.Tag
	err := r.db.First(&tag, id).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// DeleteTag deletes a tag.
func (r *Repository) DeleteTag(id uint) error {
	return r.db.Delete(&entities.Tag{}, id).Error
}

// IsTagOrphan checks if a tag has no associated books or highlights.
func (r *Repository) IsTagOrphan(tagID uint) (bool, error) {
	var bookCount int64
	if err := r.db.Table("book_tags").Where("tag_id = ?", tagID).Count(&bookCount).Error; err != nil {
		return false, err
	}
	if bookCount > 0 {
		return false, nil
	}

	var highlightCount int64
	if err := r.db.Table("highlight_tags").Where("tag_id = ?", tagID).Count(&highlightCount).Error; err != nil {
		return false, err
	}
	return highlightCount == 0, nil
}

// DeleteTagIfOrphan deletes a tag if it has no associations.
func (r *Repository) DeleteTagIfOrphan(tagID uint) error {
	orphan, err := r.IsTagOrphan(tagID)
	if err != nil {
		return err
	}
	if orphan {
		return r.DeleteTag(tagID)
	}
	return nil
}

// DeleteOrphanTags removes all orphan tags.
func (r *Repository) DeleteOrphanTags() (int64, error) {
	result := r.db.Exec(`
		DELETE FROM tags
		WHERE id NOT IN (SELECT tag_id FROM book_tags)
		AND id NOT IN (SELECT tag_id FROM highlight_tags)
	`)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// AddTagToHighlight associates a tag with a highlight.
func (r *Repository) AddTagToHighlight(highlightID, tagID uint) error {
	var highlight entities.Highlight
	if err := r.db.First(&highlight, highlightID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := r.db.First(&tag, tagID).Error; err != nil {
		return err
	}
	return r.db.Model(&highlight).Association("Tags").Append(&tag)
}

// RemoveTagFromHighlight removes a tag from a highlight.
func (r *Repository) RemoveTagFromHighlight(highlightID, tagID uint) error {
	var highlight entities.Highlight
	if err := r.db.First(&highlight, highlightID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := r.db.First(&tag, tagID).Error; err != nil {
		return err
	}
	if err := r.db.Model(&highlight).Association("Tags").Delete(&tag); err != nil {
		return err
	}
	return r.DeleteTagIfOrphan(tagID)
}

// AddTagToBook associates a tag with a book.
func (r *Repository) AddTagToBook(bookID, tagID uint) error {
	var book entities.Book
	if err := r.db.First(&book, bookID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := r.db.First(&tag, tagID).Error; err != nil {
		return err
	}
	return r.db.Model(&book).Association("Tags").Append(&tag)
}

// RemoveTagFromBook removes a tag from a book.
func (r *Repository) RemoveTagFromBook(bookID, tagID uint) error {
	var book entities.Book
	if err := r.db.First(&book, bookID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := r.db.First(&tag, tagID).Error; err != nil {
		return err
	}
	if err := r.db.Model(&book).Association("Tags").Delete(&tag); err != nil {
		return err
	}
	return r.DeleteTagIfOrphan(tagID)
}

// GetBooksByTag retrieves books that have a specific tag.
func (r *Repository) GetBooksByTag(tagID uint, userID uint) ([]entities.Book, error) {
	var tag entities.Tag
	if err := r.db.First(&tag, tagID).Error; err != nil {
		return nil, err
	}

	var books []entities.Book

	subQuery := `
		books.id IN (
			SELECT book_id FROM book_tags WHERE tag_id = ?
		) OR books.id IN (
			SELECT book_id FROM highlights
			JOIN highlight_tags ON highlights.id = highlight_tags.highlight_id
			WHERE highlight_tags.tag_id = ?
		)
	`

	query := r.db.
		Preload("Highlights", func(db *gorm.DB) *gorm.DB {
			return db.Order("location_value ASC, highlighted_at ASC")
		}).
		Preload("Source").
		Preload("Tags").
		Where(subQuery, tagID, tagID)

	if userID > 0 {
		query = query.Where("books.user_id = ?", userID)
	}

	err := query.Find(&books).Error
	return books, err
}

// GetBookByID retrieves a book by ID (for TagStore interface).
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

// GetHighlightByID retrieves a highlight by ID (for TagStore interface).
func (r *Repository) GetHighlightByID(id uint) (*entities.Highlight, error) {
	var highlight entities.Highlight
	err := r.db.Preload("Tags").Preload("Source").First(&highlight, id).Error
	if err != nil {
		return nil, err
	}
	return &highlight, nil
}
