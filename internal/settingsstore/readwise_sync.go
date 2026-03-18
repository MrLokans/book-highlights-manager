package settingsstore

import (
	"strconv"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

// Readwise sync setting descriptors
var (
	readwiseSyncEnabled = Setting{
		Key:     entities.SettingKeyReadwiseSyncEnabled,
		EnvVars: []string{"READWISE_SYNC_ENABLED"},
		Default: "false",
	}
	readwiseSyncToken = Setting{
		Key:     entities.SettingKeyReadwiseSyncToken,
		EnvVars: []string{"READWISE_TOKEN"},
		Default: "",
	}
	readwiseSyncSchedule = Setting{
		Key:     entities.SettingKeyReadwiseSyncSchedule,
		EnvVars: []string{"READWISE_SYNC_SCHEDULE"},
		Default: "0 */6 * * *",
	}
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

// GetReadwiseSyncEnabled returns the current setting value.
func (s *SettingsStore) GetReadwiseSyncEnabled() bool { return s.GetBool(readwiseSyncEnabled) }

// GetReadwiseSyncEnabledSource returns the configuration source (database, env, or default).
func (s *SettingsStore) GetReadwiseSyncEnabledSource() string {
	return s.GetSource(readwiseSyncEnabled)
}

// SetReadwiseSyncEnabled updates the setting value.
func (s *SettingsStore) SetReadwiseSyncEnabled(enabled bool) error {
	return s.SetBool(readwiseSyncEnabled, enabled)
}

// GetReadwiseSyncToken returns the current setting value.
func (s *SettingsStore) GetReadwiseSyncToken() string { return s.Get(readwiseSyncToken) }

// GetReadwiseSyncTokenSource returns the configuration source (database, env, or default).
func (s *SettingsStore) GetReadwiseSyncTokenSource() string { return s.GetSource(readwiseSyncToken) }

// HasReadwiseSyncToken returns whether the setting has a non-empty value.
func (s *SettingsStore) HasReadwiseSyncToken() bool { return s.GetReadwiseSyncToken() != "" }

// SetReadwiseSyncToken updates the setting value.
func (s *SettingsStore) SetReadwiseSyncToken(token string) error {
	return s.Set(readwiseSyncToken, token)
}

// GetReadwiseSyncSchedule returns the current setting value.
func (s *SettingsStore) GetReadwiseSyncSchedule() string { return s.Get(readwiseSyncSchedule) }

// GetReadwiseSyncScheduleSource returns the configuration source (database, env, or default).
func (s *SettingsStore) GetReadwiseSyncScheduleSource() string {
	return s.GetSource(readwiseSyncSchedule)
}

// SetReadwiseSyncSchedule updates the setting value.
func (s *SettingsStore) SetReadwiseSyncSchedule(schedule string) error {
	return s.Set(readwiseSyncSchedule, schedule)
}

// GetReadwiseSyncConfig returns the combined sync configuration.
func (s *SettingsStore) GetReadwiseSyncConfig() ReadwiseSyncConfig {
	return ReadwiseSyncConfig{
		Enabled:  s.GetReadwiseSyncEnabled(),
		Token:    s.GetReadwiseSyncToken(),
		Schedule: s.GetReadwiseSyncSchedule(),
	}
}

// GetReadwiseSyncConfigInfo returns configuration with source metadata.
func (s *SettingsStore) GetReadwiseSyncConfigInfo() ReadwiseSyncConfigInfo {
	token := s.GetReadwiseSyncToken()
	return ReadwiseSyncConfigInfo{
		Enabled:        s.GetReadwiseSyncEnabled(),
		EnabledSource:  s.GetReadwiseSyncEnabledSource(),
		Token:          maskToken(token),
		TokenSource:    s.GetReadwiseSyncTokenSource(),
		HasToken:       token != "",
		Schedule:       s.GetReadwiseSyncSchedule(),
		ScheduleSource: s.GetReadwiseSyncScheduleSource(),
	}
}

// GetReadwiseSyncStatus returns the last sync status and timestamp.
func (s *SettingsStore) GetReadwiseSyncStatus() ReadwiseSyncStatus {
	status := ReadwiseSyncStatus{}

	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncLastAt); err == nil && setting.Value != "" {
		if ts, err := time.Parse(time.RFC3339, setting.Value); err == nil {
			status.LastSyncAt = &ts
		}
	}
	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncLastStatus); err == nil {
		status.Status = setting.Value
	}
	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncLastMessage); err == nil {
		status.Message = setting.Value
	}
	if setting, err := s.db.GetSetting(entities.SettingKeyReadwiseSyncHighlightsSynced); err == nil && setting.Value != "" {
		if count, err := strconv.Atoi(setting.Value); err == nil {
			status.HighlightsSynced = count
		}
	}

	return status
}

// SetReadwiseSyncStatus updates the last sync status.
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

// ClearReadwiseSyncSettings removes all related settings from the database.
func (s *SettingsStore) ClearReadwiseSyncSettings() error {
	return s.Clear(readwiseSyncEnabled, readwiseSyncToken, readwiseSyncSchedule)
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
