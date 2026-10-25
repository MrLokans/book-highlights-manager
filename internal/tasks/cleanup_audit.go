// Package tasks implements background job processing with backlite.
package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mrlokans/assistant/internal/audit"

	"github.com/mikestefanello/backlite"
)

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
func CleanupAuditEventsProcessor(cleaner *audit.Service) backlite.QueueProcessor[CleanupAuditEventsTask] {
	return func(_ context.Context, task CleanupAuditEventsTask) error {
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

		slog.Info("Task cleaned up audit events", "deleted", deleted, "retention_days", retentionDays)
		return nil
	}
}

// NewCleanupAuditEventsQueue creates a backlite queue for audit cleanup tasks.
func NewCleanupAuditEventsQueue(cleaner *audit.Service) backlite.Queue {
	return backlite.NewQueue(CleanupAuditEventsProcessor(cleaner))
}
