package entities

import "time"

type AuditEventType string

const (
	AuditEventImport         AuditEventType = "import"
	AuditEventExport         AuditEventType = "export"
	AuditEventDelete         AuditEventType = "delete"
	AuditEventMetadataEnrich AuditEventType = "metadata_enrich"
	AuditEventSync           AuditEventType = "sync"
	AuditEventAuth           AuditEventType = "auth"
	AuditEventSettings       AuditEventType = "settings"
)

type AuditStatus string

const (
	AuditStatusSuccess AuditStatus = "success"
	AuditStatusFailed  AuditStatus = "failed"
)

type AuditEvent struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"index" json:"user_id"`
	EventType   AuditEventType `gorm:"index;size:50" json:"event_type"`
	Action      string         `gorm:"size:100" json:"action"`      // e.g., "kindle_import", "book_delete"
	Description string         `gorm:"size:500" json:"description"` // Human-readable summary
	EntityType  string         `gorm:"size:50" json:"entity_type"`  // "book", "highlight", etc.
	EntityID    *uint          `gorm:"index" json:"entity_id,omitempty"`
	Metadata    string         `gorm:"type:text" json:"metadata,omitempty"` // JSON for extra data
	IPAddress   string         `gorm:"size:45" json:"ip_address,omitempty"`
	UserAgent   string         `gorm:"size:500" json:"user_agent,omitempty"`
	Status      AuditStatus    `gorm:"size:20" json:"status"`
	ErrorMsg    string         `gorm:"size:500" json:"error_msg,omitempty"`
	CreatedAt   time.Time      `gorm:"index" json:"created_at"`
}

func (AuditEvent) TableName() string {
	return "audit_events"
}
