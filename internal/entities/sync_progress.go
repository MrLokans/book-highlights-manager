package entities

import (
	"time"
)

// SyncType identifies which background synchronization process is being tracked.
type SyncType string

// Supported sync types.
const (
	SyncTypeMetadata SyncType = "metadata"
)

// SyncStatus represents the current state of a sync operation.
type SyncStatus string

// Sync status values.
const (
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

// SyncProgress tracks the progress and outcome of a background sync operation.
type SyncProgress struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	SyncType    SyncType   `gorm:"size:50;uniqueIndex" json:"sync_type"`
	Status      SyncStatus `gorm:"size:20" json:"status"`
	TotalItems  int        `json:"total_items"`
	Processed   int        `json:"processed"`
	Succeeded   int        `json:"succeeded"`
	Failed      int        `json:"failed"`
	Skipped     int        `json:"skipped"`
	CurrentItem string     `gorm:"size:512" json:"current_item,omitempty"`
	Error       string     `gorm:"type:text" json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TableName implements gorm.Tabler.
func (SyncProgress) TableName() string {
	return "sync_progress"
}
