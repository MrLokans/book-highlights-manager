package tasks

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mikestefanello/backlite"
)

// AuditEventCleaner provides the ability to delete old audit events.
type AuditEventCleaner interface {
	DeleteOldEvents(retention time.Duration) (int64, error)
}

// CleanupAuditEventsTask removes audit events older than the configured retention period.
type CleanupAuditEventsTask struct {
	RetentionDays int `json:"retention_days"`
}

// Config returns the queue configuration for audit cleanup tasks.
func (t CleanupAuditEventsTask) Config() backlite.QueueConfig {
	return backlite.QueueConfig{
		Name:        "cleanup_audit_events",
		MaxAttempts: 3,
		Backoff:     5 * time.Minute,
		Timeout:     2 * time.Minute,
		Retention: &backlite.Retention{
			Duration:   24 * time.Hour,
			OnlyFailed: false,
			Data:       &backlite.RetainData{OnlyFailed: true},
		},
	}
}

// CleanupAuditEventsProcessor creates a processor function for CleanupAuditEventsTask.
func CleanupAuditEventsProcessor(cleaner AuditEventCleaner) backlite.QueueProcessor[CleanupAuditEventsTask] {
	return func(ctx context.Context, task CleanupAuditEventsTask) error {
		if cleaner == nil {
			return fmt.Errorf("audit event cleaner not configured")
		}

		retentionDays := task.RetentionDays
		if retentionDays <= 0 {
			retentionDays = 30
		}
		retention := time.Duration(retentionDays) * 24 * time.Hour

		deleted, err := cleaner.DeleteOldEvents(retention)
		if err != nil {
			return fmt.Errorf("cleanup audit events: %w", err)
		}

		log.Printf("[TASK] Cleaned up %d audit events older than %d days", deleted, retentionDays)
		return nil
	}
}

// NewCleanupAuditEventsQueue creates a backlite queue for audit cleanup tasks.
func NewCleanupAuditEventsQueue(cleaner AuditEventCleaner) backlite.Queue {
	return backlite.NewQueue(CleanupAuditEventsProcessor(cleaner))
}
