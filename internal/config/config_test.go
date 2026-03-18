package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Note: NewConfig uses viper.AutomaticEnv(), so tests that check defaults
// must explicitly clear env vars that might be set in .env-local.

func TestNewConfig_ReturnsNonNil(t *testing.T) {
	cfg := NewConfig()
	assert.NotNil(t, cfg)
}

func TestNewConfig_HTTPDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("HOST", "")

	cfg := NewConfig()

	assert.Equal(t, int32(8188), cfg.Port)
	assert.Equal(t, "0.0.0.0", cfg.Host)
}

func TestNewConfig_GlobalDefaults(t *testing.T) {
	t.Setenv("SHUTDOWN_TIMEOUT_IN_SECONDS", "")

	cfg := NewConfig()

	assert.Equal(t, 2, cfg.ShutdownTimeoutInSeconds)
}

func TestNewConfig_DatabaseDefaults(t *testing.T) {
	t.Setenv("DATABASE_PATH", "")

	cfg := NewConfig()

	assert.Equal(t, DefaultDatabasePath, cfg.Path)
}

func TestNewConfig_AuditDefaults(t *testing.T) {
	t.Setenv("AUDIT_DIR", "")
	t.Setenv("AUDIT_RETENTION_DAYS", "")

	cfg := NewConfig()

	assert.Equal(t, "./audit", cfg.Dir)
	assert.Equal(t, 30, cfg.RetentionDays)
}

func TestNewConfig_UIDefaults(t *testing.T) {
	t.Setenv("TEMPLATES_PATH", "")
	t.Setenv("STATIC_PATH", "")

	cfg := NewConfig()

	assert.Equal(t, "./templates", cfg.TemplatesPath)
	assert.Equal(t, "./static", cfg.StaticPath)
}

func TestNewConfig_MoonReaderDefaults(t *testing.T) {
	t.Setenv("MOONREADER_DROPBOX_PATH", "")
	t.Setenv("MOONREADER_DATABASE_PATH", "")
	t.Setenv("MOONREADER_OUTPUT_DIR", "")

	cfg := NewConfig()

	assert.Equal(t, "/Apps/Books/.Moon+/Backup", cfg.DropboxPath)
	assert.Equal(t, DefaultMoonReaderDatabasePath, cfg.DatabasePath)
	assert.Equal(t, "./markdown", cfg.OutputDir)
}

func TestNewConfig_SyncDefaults(t *testing.T) {
	t.Setenv("OBSIDIAN_SYNC_ENABLED", "")
	t.Setenv("OBSIDIAN_SYNC_SCHEDULE", "")
	t.Setenv("READWISE_SYNC_ENABLED", "")
	t.Setenv("READWISE_SYNC_SCHEDULE", "")

	cfg := NewConfig()

	assert.False(t, cfg.ObsidianSync.Enabled)
	assert.Equal(t, "0 * * * *", cfg.ObsidianSync.Schedule)
	assert.False(t, cfg.ReadwiseSync.Enabled)
	assert.Equal(t, "0 */6 * * *", cfg.ReadwiseSync.Schedule)
}

func TestNewConfig_DemoDefaults(t *testing.T) {
	t.Setenv("DEMO_MODE", "")
	t.Setenv("DEMO_DB_PATH", "")
	t.Setenv("DEMO_USE_EMBEDDED", "")
	t.Setenv("DEMO_COVERS_PATH", "")

	cfg := NewConfig()

	assert.False(t, cfg.Demo.Enabled)
	assert.Equal(t, "./demo/demo.db", cfg.DBPath)
	assert.False(t, cfg.UseEmbedded)
	assert.Equal(t, "./demo/covers", cfg.CoversPath)
}

func TestNewConfig_PlausibleDefaults(t *testing.T) {
	t.Setenv("PLAUSIBLE_DOMAIN", "")
	t.Setenv("PLAUSIBLE_SCRIPT_URL", "")
	t.Setenv("PLAUSIBLE_EXTENSIONS", "")

	cfg := NewConfig()

	assert.Empty(t, cfg.Domain)
	assert.Equal(t, "https://plausible.io/js/script.js", cfg.ScriptURL)
	assert.Empty(t, cfg.Extensions)
}

func TestNewConfig_AuthDefaults(t *testing.T) {
	t.Setenv("AUTH_MODE", "")
	t.Setenv("AUTH_SESSION_SECRET", "")
	t.Setenv("AUTH_BCRYPT_COST", "")
	t.Setenv("AUTH_SECURE_COOKIES", "")
	t.Setenv("AUTH_MAX_LOGIN_ATTEMPTS", "")
	t.Setenv("AUTH_RATE_LIMIT_WINDOW", "")
	t.Setenv("AUTH_LOCKOUT_DURATION", "")

	cfg := NewConfig()

	assert.Equal(t, AuthModeNone, cfg.Mode)
	assert.Empty(t, cfg.SessionSecret)
	assert.Equal(t, 12, cfg.BcryptCost)
	assert.True(t, cfg.SecureCookies)
	assert.Equal(t, 5, cfg.MaxLoginAttempts)
}

func TestNewConfig_TaskDefaults(t *testing.T) {
	t.Setenv("TASKS_ENABLED", "")
	t.Setenv("TASK_WORKERS", "")
	t.Setenv("TASK_MAX_RETRIES", "")

	cfg := NewConfig()

	assert.True(t, cfg.Tasks.Enabled)
	assert.Equal(t, 2, cfg.Workers)
	assert.Equal(t, 3, cfg.MaxRetries)
}

func TestNewConfig_OAuth2Defaults(t *testing.T) {
	t.Setenv("OAUTH2_REFRESH_ENABLED", "")

	cfg := NewConfig()

	assert.True(t, cfg.RefreshEnabled)
}

func TestNewConfig_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("DATABASE_PATH", "/tmp/test.db")
	t.Setenv("READWISE_TOKEN", "my-token")
	t.Setenv("AUDIT_DIR", "/tmp/audit")
	t.Setenv("AUDIT_RETENTION_DAYS", "7")
	t.Setenv("AUTH_MODE", "local")
	t.Setenv("AUTH_BCRYPT_COST", "10")
	t.Setenv("AUTH_SECURE_COOKIES", "false")
	t.Setenv("DEMO_MODE", "true")
	t.Setenv("TASKS_ENABLED", "false")
	t.Setenv("TASK_WORKERS", "8")
	t.Setenv("DROPBOX_APP_KEY", "my-key")
	t.Setenv("PLAUSIBLE_DOMAIN", "demo.example.com")

	cfg := NewConfig()

	assert.Equal(t, int32(9999), cfg.Port)
	assert.Equal(t, "127.0.0.1", cfg.Host)
	assert.Equal(t, "/tmp/test.db", cfg.Path)
	assert.Equal(t, "my-token", cfg.Token)
	assert.Equal(t, "/tmp/audit", cfg.Dir)
	assert.Equal(t, 7, cfg.RetentionDays)
	assert.Equal(t, AuthModeLocal, cfg.Mode)
	assert.Equal(t, 10, cfg.BcryptCost)
	assert.False(t, cfg.SecureCookies)
	assert.True(t, cfg.Demo.Enabled)
	assert.False(t, cfg.Tasks.Enabled)
	assert.Equal(t, 8, cfg.Workers)
	assert.Equal(t, "my-key", cfg.AppKey)
	assert.Equal(t, "demo.example.com", cfg.Domain)
}

func TestNewConfig_ObsidianExportDir_NewEnvVar(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "/new/path")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")

	cfg := NewConfig()

	assert.Equal(t, "/new/path", cfg.ExportDir)
}

func TestNewConfig_ObsidianExportDir_LegacyFallback(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "/legacy/path")

	cfg := NewConfig()

	assert.Equal(t, "/legacy/path", cfg.ExportDir)
}

func TestNewConfig_ObsidianExportDir_NewTakesPrecedence(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "/new/path")
	t.Setenv("OBSIDIAN_VAULT_DIR", "/legacy/path")

	cfg := NewConfig()

	assert.Equal(t, "/new/path", cfg.ExportDir)
}

func TestNewConfig_EmptyWithoutEnv(t *testing.T) {
	t.Setenv("READWISE_TOKEN", "")
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")
	t.Setenv("DROPBOX_APP_KEY", "")

	cfg := NewConfig()

	assert.Empty(t, cfg.Token)
	assert.Empty(t, cfg.ExportDir)
	assert.Empty(t, cfg.AppKey)
}

func TestAuthMode_Constants(t *testing.T) {
	assert.Equal(t, AuthMode("none"), AuthModeNone)
	assert.Equal(t, AuthMode("local"), AuthModeLocal)
}
