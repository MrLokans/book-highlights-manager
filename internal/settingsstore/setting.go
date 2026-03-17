package settingsstore

import (
	"os"
	"strconv"
)

// Setting describes a single configuration value with its DB key,
// environment variable fallbacks, and default value.
//
// Resolution order: database → environment (first non-empty EnvVar wins) → Default.
type Setting struct {
	Key     string
	EnvVars []string // checked in order; first non-empty value wins
	Default string
}

// Get resolves the setting value: database → environment → default.
func (s *SettingsStore) Get(setting Setting) string {
	if val, err := s.db.GetSetting(setting.Key); err == nil && val.Value != "" {
		return val.Value
	}
	for _, env := range setting.EnvVars {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return setting.Default
}

// GetSource returns where the current value comes from: "database", "environment", or "default".
func (s *SettingsStore) GetSource(setting Setting) string {
	if val, err := s.db.GetSetting(setting.Key); err == nil && val.Value != "" {
		return "database"
	}
	for _, env := range setting.EnvVars {
		if v := os.Getenv(env); v != "" {
			return "environment"
		}
	}
	return "default"
}

// Set stores a string value in the database.
func (s *SettingsStore) Set(setting Setting, value string) error {
	return s.db.SetSetting(setting.Key, value)
}

// GetBool resolves a boolean setting ("true"/"1" → true, anything else → false).
func (s *SettingsStore) GetBool(setting Setting) bool {
	v := s.Get(setting)
	return v == "true" || v == "1"
}

// SetBool stores a boolean value in the database.
func (s *SettingsStore) SetBool(setting Setting, value bool) error {
	return s.db.SetSetting(setting.Key, strconv.FormatBool(value))
}

// Clear removes the database override for the given settings, reverting to env/default.
func (s *SettingsStore) Clear(settings ...Setting) error {
	for _, setting := range settings {
		if err := s.db.DeleteSetting(setting.Key); err != nil {
			continue // ignore not-found errors
		}
	}
	return nil
}
