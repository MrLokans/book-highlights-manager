package analytics

import (
	"html/template"
	"os"
	"slices"
	"strings"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
)

// PlausibleConfig holds the effective Plausible Analytics configuration
type PlausibleConfig struct {
	Enabled    bool
	Domain     string
	ScriptURL  string
	Extensions []string
}

// PlausibleSettingsInfo contains settings with source information for UI display
type PlausibleSettingsInfo struct {
	Enabled          bool
	EnabledSource    string // "database", "environment", or "default"
	Domain           string
	DomainSource     string
	ScriptURL        string
	ScriptURLSource  string
	Extensions       []string
	ExtensionsSource string
}

// PlausibleStore handles Plausible settings with priority: database > environment > default
type PlausibleStore struct {
	db        *database.Database
	envConfig config.Plausible
}

func NewPlausibleStore(db *database.Database, envConfig config.Plausible) *PlausibleStore {
	return &PlausibleStore{
		db:        db,
		envConfig: envConfig,
	}
}

// GetEffectiveConfig returns the merged configuration with database taking priority
func (s *PlausibleStore) GetEffectiveConfig() *PlausibleConfig {
	info := s.GetSettingsInfo()

	return &PlausibleConfig{
		Enabled:    info.Enabled,
		Domain:     info.Domain,
		ScriptURL:  info.ScriptURL,
		Extensions: info.Extensions,
	}
}

// GetSettingsInfo returns settings with source information for UI display
func (s *PlausibleStore) GetSettingsInfo() PlausibleSettingsInfo {
	info := PlausibleSettingsInfo{}

	// Enabled
	info.Enabled, info.EnabledSource = s.getEnabled()

	// Domain
	info.Domain, info.DomainSource = s.getDomain()

	// ScriptURL
	info.ScriptURL, info.ScriptURLSource = s.getScriptURL()

	// Extensions
	info.Extensions, info.ExtensionsSource = s.getExtensions()

	return info
}

func (s *PlausibleStore) getEnabled() (bool, string) {
	// Database first
	setting, err := s.db.GetSetting(entities.SettingKeyPlausibleEnabled)
	if err == nil && setting.Value != "" {
		return setting.Value == "true", "database"
	}

	// Environment: enabled if domain is set
	if s.envConfig.Domain != "" {
		return true, "environment"
	}

	return false, "default"
}

func (s *PlausibleStore) getDomain() (string, string) {
	// Database first
	setting, err := s.db.GetSetting(entities.SettingKeyPlausibleDomain)
	if err == nil && setting.Value != "" {
		return setting.Value, "database"
	}

	// Environment
	if s.envConfig.Domain != "" {
		return s.envConfig.Domain, "environment"
	}

	return "", "default"
}

func (s *PlausibleStore) getScriptURL() (string, string) {
	// Database first
	setting, err := s.db.GetSetting(entities.SettingKeyPlausibleScriptURL)
	if err == nil && setting.Value != "" {
		return setting.Value, "database"
	}

	// Environment
	if s.envConfig.ScriptURL != "" {
		return s.envConfig.ScriptURL, "environment"
	}

	return "https://plausible.io/js/script.js", "default"
}

func (s *PlausibleStore) getExtensions() ([]string, string) {
	// Database first
	setting, err := s.db.GetSetting(entities.SettingKeyPlausibleExtensions)
	if err == nil && setting.Value != "" {
		return parseExtensions(setting.Value), "database"
	}

	// Environment
	if s.envConfig.Extensions != "" {
		return parseExtensions(s.envConfig.Extensions), "environment"
	}

	return []string{}, "default"
}

// SetEnabled sets the enabled flag in the database
func (s *PlausibleStore) SetEnabled(enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	return s.db.SetSetting(entities.SettingKeyPlausibleEnabled, value)
}

// SetDomain sets the domain in the database
func (s *PlausibleStore) SetDomain(domain string) error {
	return s.db.SetSetting(entities.SettingKeyPlausibleDomain, domain)
}

// SetScriptURL sets the script URL in the database
func (s *PlausibleStore) SetScriptURL(url string) error {
	return s.db.SetSetting(entities.SettingKeyPlausibleScriptURL, url)
}

// SetExtensions sets the extensions in the database
func (s *PlausibleStore) SetExtensions(extensions []string) error {
	return s.db.SetSetting(entities.SettingKeyPlausibleExtensions, strings.Join(extensions, ","))
}

// ClearSettings removes all Plausible settings from the database, reverting to env/defaults
func (s *PlausibleStore) ClearSettings() error {
	keys := []string{
		entities.SettingKeyPlausibleEnabled,
		entities.SettingKeyPlausibleDomain,
		entities.SettingKeyPlausibleScriptURL,
		entities.SettingKeyPlausibleExtensions,
	}

	for _, key := range keys {
		if err := s.db.DeleteSetting(key); err != nil {
			// Ignore "not found" errors
			if !strings.Contains(err.Error(), "record not found") {
				return err
			}
		}
	}
	return nil
}

// BuildScriptURL constructs the Plausible script URL with extensions
func BuildScriptURL(baseURL string, extensions []string) string {
	if len(extensions) == 0 {
		return baseURL
	}

	// Plausible extension format: script.ext1.ext2.js
	// Base: https://plausible.io/js/script.js
	// With extensions: https://plausible.io/js/script.outbound-links.file-downloads.js

	// Insert extensions before .js suffix
	if base, found := strings.CutSuffix(baseURL, ".js"); found {
		return base + "." + strings.Join(extensions, ".") + ".js"
	}

	return baseURL
}

// GenerateScriptTag returns safe HTML for the Plausible script tag
func GenerateScriptTag(cfg *PlausibleConfig) template.HTML {
	if !cfg.Enabled || cfg.Domain == "" {
		return ""
	}

	scriptURL := BuildScriptURL(cfg.ScriptURL, cfg.Extensions)

	return template.HTML(`<script defer data-domain="` + template.HTMLEscapeString(cfg.Domain) + `" src="` + template.HTMLEscapeString(scriptURL) + `"></script>`)
}

// parseExtensions splits comma-separated extensions and trims whitespace
func parseExtensions(s string) []string {
	if s == "" {
		return []string{}
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// ValidExtensions lists the known Plausible script extensions
var ValidExtensions = []string{
	"outbound-links",
	"file-downloads",
	"tagged-events",
	"hash",
	"compat",
	"local",
	"manual",
	"pageview-props",
	"revenue",
}

// IsValidExtension checks if an extension is known
func IsValidExtension(ext string) bool {
	return slices.Contains(ValidExtensions, ext)
}

// GetEffectiveConfigFromEnvOnly returns config based only on environment variables
// Used during app startup before database is available
func GetEffectiveConfigFromEnvOnly() *PlausibleConfig {
	domain := os.Getenv("PLAUSIBLE_DOMAIN")
	scriptURL := os.Getenv("PLAUSIBLE_SCRIPT_URL")
	if scriptURL == "" {
		scriptURL = "https://plausible.io/js/script.js"
	}
	extensions := parseExtensions(os.Getenv("PLAUSIBLE_EXTENSIONS"))

	return &PlausibleConfig{
		Enabled:    domain != "",
		Domain:     domain,
		ScriptURL:  scriptURL,
		Extensions: extensions,
	}
}
