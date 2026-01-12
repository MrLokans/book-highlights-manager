package entities

import (
	"time"
)

type SyncType string

const (
	SyncTypeMetadata SyncType = "metadata"
)

type SyncStatus string

const (
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

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

func (SyncProgress) TableName() string {
	return "sync_progress"
}
