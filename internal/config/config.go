package config

import (
	"github.com/spf13/viper"
)

type (
	// Config -.
	Config struct {
		HTTP
		Obsidian
		Audit
		Global
		Readwise
		Database
		UI
		Dropbox
		MoonReader
	}

	// App -.
	HTTP struct {
		Port int32
		Host string
	}
	Obsidian struct {
		VaultDir   string
		ExportPath string
	}
	Audit struct {
		Dir string
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
)

func NewConfig() *Config {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("port", 8080)
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("shutdown_timeout_in_seconds", 2)
	v.SetDefault("obsidian_export_path", "data")
	v.SetDefault("obsidian_vault_dir", "")
	v.SetDefault("database_path", "./highlights-manager.db")
	v.SetDefault("audit_dir", "./audit")
	v.SetDefault("templates_path", "./templates")
	v.SetDefault("static_path", "./static")
	v.SetDefault("moonreader_dropbox_path", "/Apps/Books/.Moon+/Backup")
	v.SetDefault("moonreader_database_path", "./moonreader.db")
	v.SetDefault("moonreader_output_dir", "./markdown")

	return &Config{
		HTTP: HTTP{
			Port: v.GetInt32("PORT"),
			Host: v.GetString("HOST"),
		},
		Obsidian: Obsidian{
			VaultDir:   v.GetString("OBSIDIAN_VAULT_DIR"),
			ExportPath: v.GetString("OBSIDIAN_EXPORT_PATH"),
		},
		Audit: Audit{
			Dir: v.GetString("AUDIT_DIR"),
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
	}
}
