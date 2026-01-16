package settingsstore

import (
	"github.com/mrlokans/assistant/internal/database"
)

// Priority: database > environment > default
type SettingsStore struct {
	db *database.Database
}

func New(db *database.Database) *SettingsStore {
	return &SettingsStore{db: db}
}
