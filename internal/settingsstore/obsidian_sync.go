package settingsstore

import (
	"os"
	"strconv"
	"time"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/robfig/cron/v3"
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

// GetObsidianSyncEnabled returns whether sync is enabled (database > env > default)
func (s *SettingsStore) GetObsidianSyncEnabled() bool {
	// Try database first
	setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncEnabled)
	if err == nil && setting.Value != "" {
		return setting.Value == "true" || setting.Value == "1"
	}

	// Try environment variable
	if envVal := os.Getenv("OBSIDIAN_SYNC_ENABLED"); envVal != "" {
		return envVal == "true" || envVal == "1"
	}

	// Default: disabled
	return false
}

// GetObsidianSyncEnabledSource returns the source of the enabled setting
func (s *SettingsStore) GetObsidianSyncEnabledSource() string {
	setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncEnabled)
	if err == nil && setting.Value != "" {
		return "database"
	}
	if envVal := os.Getenv("OBSIDIAN_SYNC_ENABLED"); envVal != "" {
		return "environment"
	}
	return "default"
}

// SetObsidianSyncEnabled saves the enabled setting to database
func (s *SettingsStore) SetObsidianSyncEnabled(enabled bool) error {
	return s.db.SetSetting(entities.SettingKeyObsidianSyncEnabled, strconv.FormatBool(enabled))
}

// GetObsidianSyncExportDir returns the export directory (database > env > "")
func (s *SettingsStore) GetObsidianSyncExportDir() string {
	// Try database first
	setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncExportDir)
	if err == nil && setting.Value != "" {
		return setting.Value
	}

	// Try new environment variable name first
	if envVal := os.Getenv("OBSIDIAN_EXPORT_DIR"); envVal != "" {
		return envVal
	}

	// Fall back to legacy env var for backward compatibility
	if envVal := os.Getenv("OBSIDIAN_VAULT_DIR"); envVal != "" {
		return envVal
	}

	// Default: empty (not configured)
	return ""
}

// GetObsidianSyncExportDirSource returns the source of the export dir setting
func (s *SettingsStore) GetObsidianSyncExportDirSource() string {
	setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncExportDir)
	if err == nil && setting.Value != "" {
		return "database"
	}
	if envVal := os.Getenv("OBSIDIAN_EXPORT_DIR"); envVal != "" {
		return "environment"
	}
	if envVal := os.Getenv("OBSIDIAN_VAULT_DIR"); envVal != "" {
		return "environment"
	}
	return "default"
}

// SetObsidianSyncExportDir saves the export directory to database
func (s *SettingsStore) SetObsidianSyncExportDir(path string) error {
	return s.db.SetSetting(entities.SettingKeyObsidianSyncExportDir, path)
}

// GetObsidianSyncSchedule returns the cron schedule (database > env > default)
func (s *SettingsStore) GetObsidianSyncSchedule() string {
	// Try database first
	setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncSchedule)
	if err == nil && setting.Value != "" {
		return setting.Value
	}

	// Try environment variable
	if envVal := os.Getenv("OBSIDIAN_SYNC_SCHEDULE"); envVal != "" {
		return envVal
	}

	// Default: hourly
	return "0 * * * *"
}

// GetObsidianSyncScheduleSource returns the source of the schedule setting
func (s *SettingsStore) GetObsidianSyncScheduleSource() string {
	setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncSchedule)
	if err == nil && setting.Value != "" {
		return "database"
	}
	if envVal := os.Getenv("OBSIDIAN_SYNC_SCHEDULE"); envVal != "" {
		return "environment"
	}
	return "default"
}

// SetObsidianSyncSchedule saves the schedule to database
func (s *SettingsStore) SetObsidianSyncSchedule(schedule string) error {
	return s.db.SetSetting(entities.SettingKeyObsidianSyncSchedule, schedule)
}

// GetObsidianSyncConfig returns the effective configuration
func (s *SettingsStore) GetObsidianSyncConfig() ObsidianSyncConfig {
	return ObsidianSyncConfig{
		Enabled:   s.GetObsidianSyncEnabled(),
		ExportDir: s.GetObsidianSyncExportDir(),
		Schedule:  s.GetObsidianSyncSchedule(),
	}
}

// GetObsidianSyncConfigInfo returns the configuration with source information
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

// GetObsidianSyncStatus returns the last sync status
func (s *SettingsStore) GetObsidianSyncStatus() ObsidianSyncStatus {
	status := ObsidianSyncStatus{}

	// Get last sync timestamp
	if setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncLastAt); err == nil && setting.Value != "" {
		if ts, err := time.Parse(time.RFC3339, setting.Value); err == nil {
			status.LastSyncAt = &ts
		}
	}

	// Get last status
	if setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncLastStatus); err == nil {
		status.Status = setting.Value
	}

	// Get last message
	if setting, err := s.db.GetSetting(entities.SettingKeyObsidianSyncLastMessage); err == nil {
		status.Message = setting.Value
	}

	return status
}

// SetObsidianSyncStatus updates the sync status
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

// ClearObsidianSyncSettings clears all database overrides, reverting to env/default
func (s *SettingsStore) ClearObsidianSyncSettings() error {
	keys := []string{
		entities.SettingKeyObsidianSyncEnabled,
		entities.SettingKeyObsidianSyncExportDir,
		entities.SettingKeyObsidianSyncSchedule,
	}
	for _, key := range keys {
		if err := s.db.DeleteSetting(key); err != nil {
			// Ignore not found errors
			continue
		}
	}
	return nil
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
