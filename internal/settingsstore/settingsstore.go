package settingsstore

import (
	"os"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"gorm.io/gorm"
)

// SettingsStore provides access to application settings with priority:
// 1. Database value (highest priority)
// 2. Environment variable
// 3. Default value (lowest priority)
type SettingsStore struct {
	db *database.Database
}

// New creates a new SettingsStore
func New(db *database.Database) *SettingsStore {
	return &SettingsStore{db: db}
}

// GetMarkdownExportPath returns the markdown export path with priority:
// 1. Database value
// 2. OBSIDIAN_VAULT_DIR environment variable
// 3. Current working directory
func (s *SettingsStore) GetMarkdownExportPath() string {
	// Try database first
	setting, err := s.db.GetSetting(entities.SettingKeyMarkdownExportPath)
	if err == nil && setting.Value != "" {
		return setting.Value
	}

	// Try environment variable
	if envPath := os.Getenv("OBSIDIAN_VAULT_DIR"); envPath != "" {
		return envPath
	}

	// Fall back to current working directory
	pwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return pwd
}

// SetMarkdownExportPath saves the markdown export path to the database
func (s *SettingsStore) SetMarkdownExportPath(path string) error {
	return s.db.SetSetting(entities.SettingKeyMarkdownExportPath, path)
}

// GetMarkdownExportPathSource returns where the current value comes from
func (s *SettingsStore) GetMarkdownExportPathSource() string {
	// Check database first
	setting, err := s.db.GetSetting(entities.SettingKeyMarkdownExportPath)
	if err == nil && setting.Value != "" {
		return "database"
	}

	// Check environment variable
	if envPath := os.Getenv("OBSIDIAN_VAULT_DIR"); envPath != "" {
		return "environment"
	}

	return "default"
}

// GetMarkdownExportPathInfo returns the path and its source
type ExportPathInfo struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "database", "environment", or "default"
}

// GetMarkdownExportPathInfo returns detailed information about the export path
func (s *SettingsStore) GetMarkdownExportPathInfo() ExportPathInfo {
	return ExportPathInfo{
		Path:   s.GetMarkdownExportPath(),
		Source: s.GetMarkdownExportPathSource(),
	}
}

// ClearMarkdownExportPath removes the database setting, falling back to env/default
func (s *SettingsStore) ClearMarkdownExportPath() error {
	err := s.db.DeleteSetting(entities.SettingKeyMarkdownExportPath)
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return err
}
