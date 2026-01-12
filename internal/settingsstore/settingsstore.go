package settingsstore

import (
	"os"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"gorm.io/gorm"
)

// Priority: database > environment > default
type SettingsStore struct {
	db *database.Database
}

func New(db *database.Database) *SettingsStore {
	return &SettingsStore{db: db}
}

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

func (s *SettingsStore) SetMarkdownExportPath(path string) error {
	return s.db.SetSetting(entities.SettingKeyMarkdownExportPath, path)
}

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

type ExportPathInfo struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "database", "environment", or "default"
}

func (s *SettingsStore) GetMarkdownExportPathInfo() ExportPathInfo {
	return ExportPathInfo{
		Path:   s.GetMarkdownExportPath(),
		Source: s.GetMarkdownExportPathSource(),
	}
}

func (s *SettingsStore) ClearMarkdownExportPath() error {
	err := s.db.DeleteSetting(entities.SettingKeyMarkdownExportPath)
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return err
}
