package settingsstore

import (
	"github.com/mrlokans/assistant/internal/database"
)

// SettingsStore provides typed access to application settings with priority: database > environment > default.
type SettingsStore struct {
	db *database.Database
}

// New creates a SettingsStore backed by the given database settings repository.
func New(db *database.Database) *SettingsStore {
	return &SettingsStore{db: db}
}
