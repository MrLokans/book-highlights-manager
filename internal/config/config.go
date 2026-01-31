package config

import (
	"time"

	"github.com/spf13/viper"
)

type AuthMode string

const (
	AuthModeNone  AuthMode = "none"  // No authentication required (default)
	AuthModeLocal AuthMode = "local" // Local user database with sessions
)

type (
	Config struct {
		HTTP
		Obsidian
		ObsidianSync
		ReadwiseSync
		Audit
		Global
		Readwise
		Database
		UI
		Dropbox
		MoonReader
		Tasks
		Auth
		Demo
		Plausible
		OAuth2
	}

	HTTP struct {
		Port int32
		Host string
	}
	Obsidian struct {
		ExportDir string // Directory for markdown exports
	}
	ObsidianSync struct {
		Enabled  bool
		Schedule string // Cron format: "0 * * * *" = hourly
	}
	ReadwiseSync struct {
		Enabled  bool
		Schedule string // Cron format: "0 */6 * * *" = every 6 hours
	}
	Audit struct {
		Dir           string
		RetentionDays int // Days to keep audit events (default: 30)
	}

	Global struct {
		ShutdownTimeoutInSeconds int
	}
	Readwise struct {
		Token string
	}
	Database struct {
		Path string
	}
	UI struct {
		TemplatesPath string
		StaticPath    string
	}
	Dropbox struct {
		AppKey string
	}
	MoonReader struct {
		DropboxPath  string
		DatabasePath string
		OutputDir    string
	}
	Tasks struct {
		Enabled           bool
		Workers           int
		MaxRetries        int
		RetryDelay        time.Duration
		TaskTimeout       time.Duration
		ReleaseAfter      time.Duration
		CleanupInterval   time.Duration
		RetentionDuration time.Duration
	}
	Auth struct {
		Mode            AuthMode
		SessionSecret   string
		SessionLifetime time.Duration
		TokenExpiry     time.Duration
		BcryptCost      int
		SecureCookies   bool // Set to false for local dev without HTTPS

		// Rate limiting configuration
		MaxLoginAttempts int           // Max failed attempts before lockout (default: 5)
		RateLimitWindow  time.Duration // Time window for counting attempts (default: 15m)
		LockoutDuration  time.Duration // How long to lock out (default: 30m)
	}
	Demo struct {
		Enabled       bool          // Enable demo mode
		DBPath        string        // Path to bundled demo database
		ResetInterval time.Duration // Interval between database resets
		UseEmbedded   bool          // Use embedded assets instead of file paths
		CoversPath    string        // Path to covers directory
	}
	Plausible struct {
		Domain     string // Domain registered in Plausible (e.g., "demo.myapp.com")
		ScriptURL  string // Script URL (default: "https://plausible.io/js/script.js")
		Extensions string // Comma-separated extensions (e.g., "outbound-links,file-downloads")
	}
	OAuth2 struct {
		RefreshEnabled bool          // Enable background token refresh
		CheckInterval  time.Duration // How often to check for expiring tokens (default: 30m)
		RefreshMargin  time.Duration // Refresh tokens expiring within this duration (default: 15m)
	}
)

// getObsidianExportDir returns the export directory, checking both new and legacy env vars
func getObsidianExportDir(v *viper.Viper) string {
	// Prefer new env var name
	if dir := v.GetString("OBSIDIAN_EXPORT_DIR"); dir != "" {
		return dir
	}
	// Fall back to legacy env var for backward compatibility
	return v.GetString("OBSIDIAN_VAULT_DIR")
}

func NewConfig() *Config {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("port", 8188)
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("shutdown_timeout_in_seconds", 2)
	v.SetDefault("obsidian_export_dir", "")
	v.SetDefault("obsidian_sync_enabled", false)
	v.SetDefault("obsidian_sync_schedule", "0 * * * *") // Hourly at :00
	v.SetDefault("readwise_sync_enabled", false)
	v.SetDefault("readwise_sync_schedule", "0 */6 * * *") // Every 6 hours
	v.SetDefault("database_path", DefaultDatabasePath)
	v.SetDefault("audit_dir", "./audit")
	v.SetDefault("audit_retention_days", 30)
	v.SetDefault("templates_path", "./templates")
	v.SetDefault("static_path", "./static")
	v.SetDefault("moonreader_dropbox_path", "/Apps/Books/.Moon+/Backup")
	v.SetDefault("moonreader_database_path", DefaultMoonReaderDatabasePath)
	v.SetDefault("moonreader_output_dir", "./markdown")

	// Demo mode defaults
	v.SetDefault("demo_mode", false)
	v.SetDefault("demo_db_path", "./demo/demo.db")
	v.SetDefault("demo_reset_interval", "15m")
	v.SetDefault("demo_use_embedded", false)
	v.SetDefault("demo_covers_path", "./demo/covers")

	// Plausible Analytics defaults
	v.SetDefault("plausible_domain", "")
	v.SetDefault("plausible_script_url", "https://plausible.io/js/script.js")
	v.SetDefault("plausible_extensions", "")

	// Auth defaults
	v.SetDefault("auth_mode", "none")
	v.SetDefault("auth_session_secret", "")       // Auto-generated if empty
	v.SetDefault("auth_session_lifetime", "24h")  // 24 hours
	v.SetDefault("auth_token_expiry", "720h")     // 30 days
	v.SetDefault("auth_bcrypt_cost", 12)          // bcrypt cost factor
	v.SetDefault("auth_secure_cookies", true)     // HTTPS-only cookies
	v.SetDefault("auth_max_login_attempts", 5)    // Max failed attempts
	v.SetDefault("auth_rate_limit_window", "15m") // Window for counting attempts
	v.SetDefault("auth_lockout_duration", "30m")  // Lockout duration

	// OAuth2 defaults
	v.SetDefault("oauth2_refresh_enabled", true)
	v.SetDefault("oauth2_check_interval", "30m")
	v.SetDefault("oauth2_refresh_margin", "15m")

	// Task queue defaults
	v.SetDefault("tasks_enabled", true)
	v.SetDefault("task_workers", 2)
	v.SetDefault("task_max_retries", 3)
	v.SetDefault("task_retry_delay", "1m")
	v.SetDefault("task_timeout", "5m")
	v.SetDefault("task_release_after", "15m")
	v.SetDefault("task_cleanup_interval", "1h")
	v.SetDefault("task_retention_duration", "24h")

	return &Config{
		HTTP: HTTP{
			Port: v.GetInt32("PORT"),
			Host: v.GetString("HOST"),
		},
		Obsidian: Obsidian{
			ExportDir: getObsidianExportDir(v),
		},
		ObsidianSync: ObsidianSync{
			Enabled:  v.GetBool("OBSIDIAN_SYNC_ENABLED"),
			Schedule: v.GetString("OBSIDIAN_SYNC_SCHEDULE"),
		},
		ReadwiseSync: ReadwiseSync{
			Enabled:  v.GetBool("READWISE_SYNC_ENABLED"),
			Schedule: v.GetString("READWISE_SYNC_SCHEDULE"),
		},
		Audit: Audit{
			Dir:           v.GetString("AUDIT_DIR"),
			RetentionDays: v.GetInt("AUDIT_RETENTION_DAYS"),
		},
		Global: Global{
			ShutdownTimeoutInSeconds: v.GetInt("SHUTDOWN_TIMEOUT_IN_SECONDS"),
		},
		Readwise: Readwise{
			Token: v.GetString("READWISE_TOKEN"),
		},
		Database: Database{
			Path: v.GetString("DATABASE_PATH"),
		},
		UI: UI{
			TemplatesPath: v.GetString("TEMPLATES_PATH"),
			StaticPath:    v.GetString("STATIC_PATH"),
		},
		Dropbox: Dropbox{
			AppKey: v.GetString("DROPBOX_APP_KEY"),
		},
		MoonReader: MoonReader{
			DropboxPath:  v.GetString("MOONREADER_DROPBOX_PATH"),
			DatabasePath: v.GetString("MOONREADER_DATABASE_PATH"),
			OutputDir:    v.GetString("MOONREADER_OUTPUT_DIR"),
		},
		Tasks: Tasks{
			Enabled:           v.GetBool("TASKS_ENABLED"),
			Workers:           v.GetInt("TASK_WORKERS"),
			MaxRetries:        v.GetInt("TASK_MAX_RETRIES"),
			RetryDelay:        v.GetDuration("TASK_RETRY_DELAY"),
			TaskTimeout:       v.GetDuration("TASK_TIMEOUT"),
			ReleaseAfter:      v.GetDuration("TASK_RELEASE_AFTER"),
			CleanupInterval:   v.GetDuration("TASK_CLEANUP_INTERVAL"),
			RetentionDuration: v.GetDuration("TASK_RETENTION_DURATION"),
		},
		Auth: Auth{
			Mode:             AuthMode(v.GetString("AUTH_MODE")),
			SessionSecret:    v.GetString("AUTH_SESSION_SECRET"),
			SessionLifetime:  v.GetDuration("AUTH_SESSION_LIFETIME"),
			TokenExpiry:      v.GetDuration("AUTH_TOKEN_EXPIRY"),
			BcryptCost:       v.GetInt("AUTH_BCRYPT_COST"),
			SecureCookies:    v.GetBool("AUTH_SECURE_COOKIES"),
			MaxLoginAttempts: v.GetInt("AUTH_MAX_LOGIN_ATTEMPTS"),
			RateLimitWindow:  v.GetDuration("AUTH_RATE_LIMIT_WINDOW"),
			LockoutDuration:  v.GetDuration("AUTH_LOCKOUT_DURATION"),
		},
		Demo: Demo{
			Enabled:       v.GetBool("DEMO_MODE"),
			DBPath:        v.GetString("DEMO_DB_PATH"),
			ResetInterval: v.GetDuration("DEMO_RESET_INTERVAL"),
			UseEmbedded:   v.GetBool("DEMO_USE_EMBEDDED"),
			CoversPath:    v.GetString("DEMO_COVERS_PATH"),
		},
		Plausible: Plausible{
			Domain:     v.GetString("PLAUSIBLE_DOMAIN"),
			ScriptURL:  v.GetString("PLAUSIBLE_SCRIPT_URL"),
			Extensions: v.GetString("PLAUSIBLE_EXTENSIONS"),
		},
		OAuth2: OAuth2{
			RefreshEnabled: v.GetBool("OAUTH2_REFRESH_ENABLED"),
			CheckInterval:  v.GetDuration("OAUTH2_CHECK_INTERVAL"),
			RefreshMargin:  v.GetDuration("OAUTH2_REFRESH_MARGIN"),
		},
	}
}
