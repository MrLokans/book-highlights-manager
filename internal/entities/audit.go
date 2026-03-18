package entities

import "time"

// AuditEventType categorizes audit log entries by the action performed.
type AuditEventType string

// Audit event types.
const (
	AuditEventImport         AuditEventType = "import"
	AuditEventExport         AuditEventType = "export"
	AuditEventDelete         AuditEventType = "delete"
	AuditEventMetadataEnrich AuditEventType = "metadata_enrich"
	AuditEventSync           AuditEventType = "sync"
	AuditEventAuth           AuditEventType = "auth"
	AuditEventSettings       AuditEventType = "settings"
)

// AuditStatus indicates whether an audited operation succeeded or failed.
type AuditStatus string

// Audit status values.
const (
	AuditStatusSuccess AuditStatus = "success"
	AuditStatusFailed  AuditStatus = "failed"
)

// AuditEvent is an immutable log entry recording a user action for traceability.
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

// TableName implements gorm.Tabler.
func (AuditEvent) TableName() string {
	return "audit_events"
}
