package settingsstore

import "github.com/mrlokans/assistant/internal/entities"

// SettingsDB provides the database operations needed by SettingsStore.
type SettingsDB interface {
	GetSetting(key string) (*entities.Setting, error)
	SetSetting(key, value string) error
	DeleteSetting(key string) error
}

// SettingsStore provides typed access to application settings with priority: database > environment > default.
type SettingsStore struct {
	db SettingsDB
}

// New creates a SettingsStore backed by the given database settings repository.
func New(db SettingsDB) *SettingsStore {
	return &SettingsStore{db: db}
}
