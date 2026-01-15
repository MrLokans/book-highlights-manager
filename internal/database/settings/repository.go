// Package settings provides database operations for application settings.
//
// # Usage
//
//	repo := settings.NewRepository(db)
//	setting, err := repo.GetSetting("theme")
package settings

import (
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all settings database operations.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new settings repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetSetting retrieves a setting by key.
func (r *Repository) GetSetting(key string) (*entities.Setting, error) {
	var setting entities.Setting
	err := r.db.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// SetSetting creates or updates a setting.
func (r *Repository) SetSetting(key, value string) error {
	var setting entities.Setting
	result := r.db.Where("key = ?", key).First(&setting)

	if result.Error == gorm.ErrRecordNotFound {
		setting = entities.Setting{
			Key:   key,
			Value: value,
		}
		return r.db.Create(&setting).Error
	} else if result.Error != nil {
		return result.Error
	}

	setting.Value = value
	return r.db.Save(&setting).Error
}

// DeleteSetting removes a setting by key.
func (r *Repository) DeleteSetting(key string) error {
	return r.db.Where("key = ?", key).Delete(&entities.Setting{}).Error
}
