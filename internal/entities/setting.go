package entities

import (
	"time"
)

type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;size:100" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string {
	return "settings"
}

// Known setting keys
const (
	// Plausible Analytics settings
	SettingKeyPlausibleEnabled    = "plausible_enabled"
	SettingKeyPlausibleDomain     = "plausible_domain"
	SettingKeyPlausibleScriptURL  = "plausible_script_url"
	SettingKeyPlausibleExtensions = "plausible_extensions"

	// Obsidian Sync settings
	SettingKeyObsidianSyncEnabled     = "obsidian_sync_enabled"
	SettingKeyObsidianSyncExportDir   = "obsidian_sync_export_dir"
	SettingKeyObsidianSyncSchedule    = "obsidian_sync_schedule"
	SettingKeyObsidianSyncLastAt      = "obsidian_sync_last_at"
	SettingKeyObsidianSyncLastStatus  = "obsidian_sync_last_status"
	SettingKeyObsidianSyncLastMessage = "obsidian_sync_last_message"
)
