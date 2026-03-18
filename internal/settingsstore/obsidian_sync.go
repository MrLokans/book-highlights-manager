// Package settingsstore provides typed access to application settings with priority: database > environment > default.
package settingsstore

import (
	"time"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/robfig/cron/v3"
)

// Obsidian sync setting descriptors
var (
	obsidianSyncEnabled = Setting{
		Key:     entities.SettingKeyObsidianSyncEnabled,
		EnvVars: []string{"OBSIDIAN_SYNC_ENABLED"},
		Default: "false",
	}
	obsidianSyncExportDir = Setting{
		Key:     entities.SettingKeyObsidianSyncExportDir,
		EnvVars: []string{"OBSIDIAN_EXPORT_DIR", "OBSIDIAN_VAULT_DIR"},
		Default: "",
	}
	obsidianSyncSchedule = Setting{
		Key:     entities.SettingKeyObsidianSyncSchedule,
		EnvVars: []string{"OBSIDIAN_SYNC_SCHEDULE"},
		Default: "0 * * * *",
	}
)

// ObsidianSyncConfig represents the effective configuration for Obsidian sync
type ObsidianSyncConfig struct {
	Enabled   bool   `json:"enabled"`
	ExportDir string `json:"export_dir"`
	Schedule  string `json:"schedule"`
}

// ObsidianSyncConfigInfo includes source information for each field
type ObsidianSyncConfigInfo struct {
	Enabled       bool   `json:"enabled"`
	EnabledSource string `json:"enabled_source"` // "database", "environment", "default"

	ExportDir       string `json:"export_dir"`
	ExportDirSource string `json:"export_dir_source"`

	Schedule       string `json:"schedule"`
	ScheduleSource string `json:"schedule_source"`
}

// ObsidianSyncStatus represents the last sync status
type ObsidianSyncStatus struct {
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	Status     string     `json:"status,omitempty"`  // "success", "failed", ""
	Message    string     `json:"message,omitempty"` // Error message or stats summary
}

// GetObsidianSyncEnabled returns whether Obsidian sync is enabled.
func (s *SettingsStore) GetObsidianSyncEnabled() bool { return s.GetBool(obsidianSyncEnabled) }

// GetObsidianSyncEnabledSource returns the config source for the enabled setting.
func (s *SettingsStore) GetObsidianSyncEnabledSource() string {
	return s.GetSource(obsidianSyncEnabled)
}

// SetObsidianSyncEnabled updates the enabled setting.
func (s *SettingsStore) SetObsidianSyncEnabled(enabled bool) error {
	return s.SetBool(obsidianSyncEnabled, enabled)
}

// GetObsidianSyncExportDir returns the configured export directory.
func (s *SettingsStore) GetObsidianSyncExportDir() string { return s.Get(obsidianSyncExportDir) }

// GetObsidianSyncExportDirSource returns the config source for the export directory.
func (s *SettingsStore) GetObsidianSyncExportDirSource() string {
	return s.GetSource(obsidianSyncExportDir)
}

// SetObsidianSyncExportDir updates the export directory setting.
func (s *SettingsStore) SetObsidianSyncExportDir(path string) error {
	return s.Set(obsidianSyncExportDir, path)
}

// GetObsidianSyncSchedule returns the cron schedule for Obsidian sync.
func (s *SettingsStore) GetObsidianSyncSchedule() string { return s.Get(obsidianSyncSchedule) }

// GetObsidianSyncScheduleSource returns the configuration source (database, env, or default).
func (s *SettingsStore) GetObsidianSyncScheduleSource() string {
	return s.GetSource(obsidianSyncSchedule)
}

// SetObsidianSyncSchedule updates the setting value.
func (s *SettingsStore) SetObsidianSyncSchedule(schedule string) error {
	return s.Set(obsidianSyncSchedule, schedule)
}

// GetObsidianSyncConfig returns the combined sync configuration.
func (s *SettingsStore) GetObsidianSyncConfig() ObsidianSyncConfig {
	return ObsidianSyncConfig{
		Enabled:   s.GetObsidianSyncEnabled(),
		ExportDir: s.GetObsidianSyncExportDir(),
		Schedule:  s.GetObsidianSyncSchedule(),
	}
}

// GetObsidianSyncConfigInfo returns configuration with source metadata.
func (s *SettingsStore) GetObsidianSyncConfigInfo() ObsidianSyncConfigInfo {
	return ObsidianSyncConfigInfo{
		Enabled:         s.GetObsidianSyncEnabled(),
		EnabledSource:   s.GetObsidianSyncEnabledSource(),
		ExportDir:       s.GetObsidianSyncExportDir(),
		ExportDirSource: s.GetObsidianSyncExportDirSource(),
		Schedule:        s.GetObsidianSyncSchedule(),
		ScheduleSource:  s.GetObsidianSyncScheduleSource(),
	}
}

// GetObsidianSyncStatus returns the last sync status and timestamp.
func (s *SettingsStore) GetObsidianSyncStatus() ObsidianSyncStatus {
	status := ObsidianSyncStatus{}

	if setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncLastAt); err == nil && setting.Value != "" {
		if ts, err := time.Parse(time.RFC3339, setting.Value); err == nil {
			status.LastSyncAt = &ts
		}
	}
	if setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncLastStatus); err == nil {
		status.Status = setting.Value
	}
	if setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncLastMessage); err == nil {
		status.Message = setting.Value
	}

	return status
}

// SetObsidianSyncStatus updates the last sync status.
func (s *SettingsStore) SetObsidianSyncStatus(status, message string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	if err := s.db.SetSetting(entities.SettingKeyObsidianSyncLastAt, now); err != nil {
		return err
	}
	if err := s.db.SetSetting(entities.SettingKeyObsidianSyncLastStatus, status); err != nil {
		return err
	}
	return s.db.SetSetting(entities.SettingKeyObsidianSyncLastMessage, message)
}

// ClearObsidianSyncSettings removes all related settings from the database.
func (s *SettingsStore) ClearObsidianSyncSettings() error {
	return s.Clear(obsidianSyncEnabled, obsidianSyncExportDir, obsidianSyncSchedule)
}

// ValidateCronSchedule validates a cron schedule string
func ValidateCronSchedule(schedule string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	return err
}

// GetCronDescription returns a human-readable description of a cron schedule
func GetCronDescription(schedule string) string {
	switch schedule {
	case "0 * * * *":
		return "Every hour at :00"
	case "*/15 * * * *":
		return "Every 15 minutes"
	case "*/30 * * * *":
		return "Every 30 minutes"
	case "0 */6 * * *":
		return "Every 6 hours"
	case "0 0 * * *":
		return "Daily at midnight"
	case "0 0 * * 0":
		return "Weekly on Sunday at midnight"
	default:
		return "Custom schedule: " + schedule
	}
}

// GetNextRunTime calculates when the next sync will run based on the schedule
func GetNextRunTime(schedule string) (*time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return nil, err
	}
	next := sched.Next(time.Now())
	return &next, nil
}

// NewObsidianSyncConfigFromEnv creates settings from environment config (for use when database not yet ready)
func NewObsidianSyncConfigFromEnv(cfg config.ObsidianSync, obsidianCfg config.Obsidian) ObsidianSyncConfig {
	return ObsidianSyncConfig{
		Enabled:   cfg.Enabled,
		ExportDir: obsidianCfg.ExportDir,
		Schedule:  cfg.Schedule,
	}
}
