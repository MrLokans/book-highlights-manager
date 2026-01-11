package entities

import (
	"time"
)

// Setting represents a key-value configuration setting stored in the database
type Setting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;size:100" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName specifies the table name for Setting
func (Setting) TableName() string {
	return "settings"
}

// Known setting keys
const (
	SettingKeyMarkdownExportPath = "markdown_export_path"
)
