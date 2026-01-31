package settingsstore

import (
	"os"
	"strconv"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

// ReadwiseSyncConfig represents the effective configuration for Readwise sync
type ReadwiseSyncConfig struct {
	Enabled  bool   `json:"enabled"`
	Token    string `json:"token"`
	Schedule string `json:"schedule"`
}

// ReadwiseSyncConfigInfo includes source information for each field
type ReadwiseSyncConfigInfo struct {
	Enabled       bool   `json:"enabled"`
	EnabledSource string `json:"enabled_source"` // "database", "environment", "default"

	Token       string `json:"token"` // Masked for display
	TokenSource string `json:"token_source"`
	HasToken    bool   `json:"has_token"` // Indicates if a token is configured

	Schedule       string `json:"schedule"`
	ScheduleSource string `json:"schedule_source"`
}

// ReadwiseSyncStatus represents the last sync status
type ReadwiseSyncStatus struct {
	LastSyncAt       *time.Time `json:"last_sync_at,omitempty"`
	Status           string     `json:"status,omitempty"`            // "success", "failed", "running", ""
	Message          string     `json:"message,omitempty"`           // Error message or stats summary
	HighlightsSynced int        `json:"highlights_synced,omitempty"` // Count from last sync
}

// GetReadwiseSyncEnabled returns whether sync is enabled (database > env > default)
func (s *SettingsStore) GetReadwiseSyncEnabled() bool {
	// Try database first
	setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncEnabled)
	if err == nil && setting.Value != "" {
		return setting.Value == "true" || setting.Value == "1"
	}

	// Try environment variable
	if envVal := os.Getenv("READWISE_SYNC_ENABLED"); envVal != "" {
		return envVal == "true" || envVal == "1"
	}

	// Default: disabled
	return false
}

// GetReadwiseSyncEnabledSource returns the source of the enabled setting
func (s *SettingsStore) GetReadwiseSyncEnabledSource() string {
	setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncEnabled)
	if err == nil && setting.Value != "" {
		return "database"
	}
	if envVal := os.Getenv("READWISE_SYNC_ENABLED"); envVal != "" {
		return "environment"
	}
	return "default"
}

// SetReadwiseSyncEnabled saves the enabled setting to database
func (s *SettingsStore) SetReadwiseSyncEnabled(enabled bool) error {
	return s.db.SetSetting(entities.SettingKeyReadwiseSyncEnabled, strconv.FormatBool(enabled))
}

// GetReadwiseSyncToken returns the API token (database > env > "")
func (s *SettingsStore) GetReadwiseSyncToken() string {
	// Try database first
	setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncToken)
	if err == nil && setting.Value != "" {
		return setting.Value
	}

	// Try environment variable
	if envVal := os.Getenv("READWISE_TOKEN"); envVal != "" {
		return envVal
	}

	// Default: empty (not configured)
	return ""
}

// GetReadwiseSyncTokenSource returns the source of the token setting
func (s *SettingsStore) GetReadwiseSyncTokenSource() string {
	setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncToken)
	if err == nil && setting.Value != "" {
		return "database"
	}
	if envVal := os.Getenv("READWISE_TOKEN"); envVal != "" {
		return "environment"
	}
	return "default"
}

// HasReadwiseSyncToken returns whether a token is configured from any source
func (s *SettingsStore) HasReadwiseSyncToken() bool {
	return s.GetReadwiseSyncToken() != ""
}

// SetReadwiseSyncToken saves the token to database
func (s *SettingsStore) SetReadwiseSyncToken(token string) error {
	return s.db.SetSetting(entities.SettingKeyReadwiseSyncToken, token)
}

// GetReadwiseSyncSchedule returns the cron schedule (database > env > default)
func (s *SettingsStore) GetReadwiseSyncSchedule() string {
	// Try database first
	setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncSchedule)
	if err == nil && setting.Value != "" {
		return setting.Value
	}

	// Try environment variable
	if envVal := os.Getenv("READWISE_SYNC_SCHEDULE"); envVal != "" {
		return envVal
	}

	// Default: every 6 hours
	return "0 */6 * * *"
}

// GetReadwiseSyncScheduleSource returns the source of the schedule setting
func (s *SettingsStore) GetReadwiseSyncScheduleSource() string {
	setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncSchedule)
	if err == nil && setting.Value != "" {
		return "database"
	}
	if envVal := os.Getenv("READWISE_SYNC_SCHEDULE"); envVal != "" {
		return "environment"
	}
	return "default"
}

// SetReadwiseSyncSchedule saves the schedule to database
func (s *SettingsStore) SetReadwiseSyncSchedule(schedule string) error {
	return s.db.SetSetting(entities.SettingKeyReadwiseSyncSchedule, schedule)
}

// GetReadwiseSyncConfig returns the effective configuration
func (s *SettingsStore) GetReadwiseSyncConfig() ReadwiseSyncConfig {
	return ReadwiseSyncConfig{
		Enabled:  s.GetReadwiseSyncEnabled(),
		Token:    s.GetReadwiseSyncToken(),
		Schedule: s.GetReadwiseSyncSchedule(),
	}
}

// GetReadwiseSyncConfigInfo returns the configuration with source information
func (s *SettingsStore) GetReadwiseSyncConfigInfo() ReadwiseSyncConfigInfo {
	token := s.GetReadwiseSyncToken()
	maskedToken := maskToken(token)

	return ReadwiseSyncConfigInfo{
		Enabled:        s.GetReadwiseSyncEnabled(),
		EnabledSource:  s.GetReadwiseSyncEnabledSource(),
		Token:          maskedToken,
		TokenSource:    s.GetReadwiseSyncTokenSource(),
		HasToken:       token != "",
		Schedule:       s.GetReadwiseSyncSchedule(),
		ScheduleSource: s.GetReadwiseSyncScheduleSource(),
	}
}

// GetReadwiseSyncStatus returns the last sync status
func (s *SettingsStore) GetReadwiseSyncStatus() ReadwiseSyncStatus {
	status := ReadwiseSyncStatus{}

	// Get last sync timestamp
	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncLastAt); err == nil && setting.Value != "" {
		if ts, err := time.Parse(time.RFC3339, setting.Value); err == nil {
			status.LastSyncAt = &ts
		}
	}

	// Get last status
	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncLastStatus); err == nil {
		status.Status = setting.Value
	}

	// Get last message
	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncLastMessage); err == nil {
		status.Message = setting.Value
	}

	// Get highlights synced count
	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncHighlightsSynced); err == nil && setting.Value != "" {
		if count, err := strconv.Atoi(setting.Value); err == nil {
			status.HighlightsSynced = count
		}
	}

	return status
}

// SetReadwiseSyncStatus updates the sync status
func (s *SettingsStore) SetReadwiseSyncStatus(status, message string, highlightsSynced int) error {
	now := time.Now().UTC().Format(time.RFC3339)

	if err := s.db.SetSetting(entities.SettingKeyReadwiseSyncLastAt, now); err != nil {
		return err
	}
	if err := s.db.SetSetting(entities.SettingKeyReadwiseSyncLastStatus, status); err != nil {
		return err
	}
	if err := s.db.SetSetting(entities.SettingKeyReadwiseSyncLastMessage, message); err != nil {
		return err
	}
	return s.db.SetSetting(entities.SettingKeyReadwiseSyncHighlightsSynced, strconv.Itoa(highlightsSynced))
}

// GetReadwiseSyncLastAt returns the last successful sync timestamp (used for incremental sync)
func (s *SettingsStore) GetReadwiseSyncLastAt() *time.Time {
	setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncLastAt)
	if err != nil || setting.Value == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, setting.Value)
	if err != nil {
		return nil
	}
	return &ts
}

// ClearReadwiseSyncSettings clears all database overrides, reverting to env/default
func (s *SettingsStore) ClearReadwiseSyncSettings() error {
	keys := []string{
		entities.SettingKeyReadwiseSyncEnabled,
		entities.SettingKeyReadwiseSyncToken,
		entities.SettingKeyReadwiseSyncSchedule,
	}
	for _, key := range keys {
		if err := s.db.DeleteSetting(key); err != nil {
			// Ignore not found errors
			continue
		}
	}
	return nil
}

// maskToken returns a masked version of the token for display
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
