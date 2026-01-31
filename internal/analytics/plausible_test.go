package analytics

import (
	"os"
	"testing"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (*PlausibleStore, func()) {
	tmpFile, err := os.CreateTemp("", "test-plausible-*.db")
	require.NoError(t, err)
	tmpFile.Close()

	db, err := database.NewDatabase(tmpFile.Name())
	require.NoError(t, err)

	envConfig := config.Plausible{
		Domain:     "",
		ScriptURL:  "https://plausible.io/js/script.js",
		Extensions: "",
	}

	store := NewPlausibleStore(db, envConfig)

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return store, cleanup
}

func TestBuildScriptURL(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		extensions []string
		expected   string
	}{
		{
			name:       "no extensions",
			baseURL:    "https://plausible.io/js/script.js",
			extensions: []string{},
			expected:   "https://plausible.io/js/script.js",
		},
		{
			name:       "single extension",
			baseURL:    "https://plausible.io/js/script.js",
			extensions: []string{"outbound-links"},
			expected:   "https://plausible.io/js/script.outbound-links.js",
		},
		{
			name:       "multiple extensions",
			baseURL:    "https://plausible.io/js/script.js",
			extensions: []string{"outbound-links", "file-downloads"},
			expected:   "https://plausible.io/js/script.outbound-links.file-downloads.js",
		},
		{
			name:       "self-hosted with extension",
			baseURL:    "https://analytics.example.com/js/script.js",
			extensions: []string{"tagged-events"},
			expected:   "https://analytics.example.com/js/script.tagged-events.js",
		},
		{
			name:       "URL without .js suffix unchanged",
			baseURL:    "https://example.com/track",
			extensions: []string{"outbound-links"},
			expected:   "https://example.com/track",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildScriptURL(tt.baseURL, tt.extensions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateScriptTag(t *testing.T) {
	tests := []struct {
		name     string
		config   *PlausibleConfig
		expected string
	}{
		{
			name: "disabled returns empty",
			config: &PlausibleConfig{
				Enabled: false,
				Domain:  "example.com",
			},
			expected: "",
		},
		{
			name: "empty domain returns empty",
			config: &PlausibleConfig{
				Enabled: true,
				Domain:  "",
			},
			expected: "",
		},
		{
			name: "basic script tag",
			config: &PlausibleConfig{
				Enabled:   true,
				Domain:    "example.com",
				ScriptURL: "https://plausible.io/js/script.js",
			},
			expected: `<script defer data-domain="example.com" src="https://plausible.io/js/script.js"></script>`,
		},
		{
			name: "script tag with extensions",
			config: &PlausibleConfig{
				Enabled:    true,
				Domain:     "demo.app",
				ScriptURL:  "https://plausible.io/js/script.js",
				Extensions: []string{"outbound-links"},
			},
			expected: `<script defer data-domain="demo.app" src="https://plausible.io/js/script.outbound-links.js"></script>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateScriptTag(tt.config)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestIsValidExtension(t *testing.T) {
	validExtensions := []string{
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

	for _, ext := range validExtensions {
		t.Run("valid_"+ext, func(t *testing.T) {
			assert.True(t, IsValidExtension(ext))
		})
	}

	invalidExtensions := []string{
		"invalid",
		"foo",
		"outbound",
		"",
	}

	for _, ext := range invalidExtensions {
		t.Run("invalid_"+ext, func(t *testing.T) {
			assert.False(t, IsValidExtension(ext))
		})
	}
}

func TestPlausibleStore_DefaultValues(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	cfg := store.GetEffectiveConfig()

	assert.False(t, cfg.Enabled)
	assert.Empty(t, cfg.Domain)
	assert.Equal(t, "https://plausible.io/js/script.js", cfg.ScriptURL)
	assert.Empty(t, cfg.Extensions)
}

func TestPlausibleStore_SetAndGetSettings(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Set enabled
	err := store.SetEnabled(true)
	require.NoError(t, err)

	// Set domain
	err = store.SetDomain("test.example.com")
	require.NoError(t, err)

	// Set script URL
	err = store.SetScriptURL("https://custom.plausible.io/js/script.js")
	require.NoError(t, err)

	// Set extensions
	err = store.SetExtensions([]string{"outbound-links", "file-downloads"})
	require.NoError(t, err)

	// Verify effective config
	cfg := store.GetEffectiveConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "test.example.com", cfg.Domain)
	assert.Equal(t, "https://custom.plausible.io/js/script.js", cfg.ScriptURL)
	assert.Equal(t, []string{"outbound-links", "file-downloads"}, cfg.Extensions)

	// Verify settings info includes sources
	info := store.GetSettingsInfo()
	assert.Equal(t, "database", info.EnabledSource)
	assert.Equal(t, "database", info.DomainSource)
	assert.Equal(t, "database", info.ScriptURLSource)
	assert.Equal(t, "database", info.ExtensionsSource)
}

func TestPlausibleStore_ClearSettings(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Set some values first
	_ = store.SetEnabled(true)
	_ = store.SetDomain("test.example.com")
	_ = store.SetScriptURL("https://custom.plausible.io/js/script.js")
	_ = store.SetExtensions([]string{"outbound-links"})

	// Clear all settings
	err := store.ClearSettings()
	require.NoError(t, err)

	// Verify defaults are back
	cfg := store.GetEffectiveConfig()
	assert.False(t, cfg.Enabled)
	assert.Empty(t, cfg.Domain)
	assert.Equal(t, "https://plausible.io/js/script.js", cfg.ScriptURL)
	assert.Empty(t, cfg.Extensions)
}

func TestPlausibleStore_EnvironmentOverride(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-plausible-env-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := database.NewDatabase(tmpFile.Name())
	require.NoError(t, err)
	defer db.Close()

	// Create store with environment config
	envConfig := config.Plausible{
		Domain:     "env.example.com",
		ScriptURL:  "https://env.plausible.io/js/script.js",
		Extensions: "outbound-links,file-downloads",
	}

	store := NewPlausibleStore(db, envConfig)

	// Without database values, should use environment
	cfg := store.GetEffectiveConfig()
	assert.True(t, cfg.Enabled) // Enabled because domain is set
	assert.Equal(t, "env.example.com", cfg.Domain)
	assert.Equal(t, "https://env.plausible.io/js/script.js", cfg.ScriptURL)
	assert.Equal(t, []string{"outbound-links", "file-downloads"}, cfg.Extensions)

	info := store.GetSettingsInfo()
	assert.Equal(t, "environment", info.EnabledSource)
	assert.Equal(t, "environment", info.DomainSource)
	assert.Equal(t, "environment", info.ScriptURLSource)
	assert.Equal(t, "environment", info.ExtensionsSource)
}

func TestPlausibleStore_DatabaseOverridesEnvironment(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-plausible-override-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := database.NewDatabase(tmpFile.Name())
	require.NoError(t, err)
	defer db.Close()

	// Create store with environment config
	envConfig := config.Plausible{
		Domain:     "env.example.com",
		ScriptURL:  "https://env.plausible.io/js/script.js",
		Extensions: "outbound-links",
	}

	store := NewPlausibleStore(db, envConfig)

	// Set database values that should override environment
	_ = store.SetDomain("db.example.com")
	_ = store.SetEnabled(false)

	cfg := store.GetEffectiveConfig()
	assert.False(t, cfg.Enabled)                                            // Database override
	assert.Equal(t, "db.example.com", cfg.Domain)                           // Database override
	assert.Equal(t, "https://env.plausible.io/js/script.js", cfg.ScriptURL) // Still from env
	assert.Equal(t, []string{"outbound-links"}, cfg.Extensions)             // Still from env

	info := store.GetSettingsInfo()
	assert.Equal(t, "database", info.EnabledSource)
	assert.Equal(t, "database", info.DomainSource)
	assert.Equal(t, "environment", info.ScriptURLSource)
	assert.Equal(t, "environment", info.ExtensionsSource)
}

func TestParseExtensions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single extension",
			input:    "outbound-links",
			expected: []string{"outbound-links"},
		},
		{
			name:     "multiple extensions",
			input:    "outbound-links,file-downloads",
			expected: []string{"outbound-links", "file-downloads"},
		},
		{
			name:     "with spaces",
			input:    "outbound-links, file-downloads, tagged-events",
			expected: []string{"outbound-links", "file-downloads", "tagged-events"},
		},
		{
			name:     "empty parts ignored",
			input:    "outbound-links,,file-downloads",
			expected: []string{"outbound-links", "file-downloads"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseExtensions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
