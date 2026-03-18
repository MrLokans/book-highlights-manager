// Package sources provides database operations for import source management.
package sources

import (
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles source lookup operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new sources repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetByName retrieves a source by its name. Returns an error if not found.
func (r *Repository) GetByName(name string) (*entities.Source, error) {
	var source entities.Source
	err := r.db.Where("name = ?", name).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// GetAll returns all registered import sources.
func (r *Repository) GetAll() ([]entities.Source, error) {
	var sources []entities.Source
	err := r.db.Find(&sources).Error
	return sources, err
}
