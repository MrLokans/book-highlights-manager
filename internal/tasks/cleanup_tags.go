package tasks

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mikestefanello/backlite"
)

// OrphanTagsCleaner provides the ability to delete orphan tags.
type OrphanTagsCleaner interface {
	DeleteOrphanTags() (int64, error)
}

// CleanupOrphanTagsTask removes tags that have no associated books or highlights.
type CleanupOrphanTagsTask struct{}

// Config returns the queue configuration for cleanup tasks.
func (t CleanupOrphanTagsTask) Config() backlite.QueueConfig {
	return backlite.QueueConfig{
		Name:        "cleanup_orphan_tags",
		MaxAttempts: 1,
		Backoff:     time.Minute,
		Timeout:     time.Minute,
		Retention: &backlite.Retention{
			Duration:   24 * time.Hour,
			OnlyFailed: false,
			Data:       &backlite.RetainData{OnlyFailed: true},
		},
	}
}

// CleanupOrphanTagsProcessor creates a processor function for CleanupOrphanTagsTask.
func CleanupOrphanTagsProcessor(cleaner OrphanTagsCleaner) backlite.QueueProcessor[CleanupOrphanTagsTask] {
	return func(ctx context.Context, task CleanupOrphanTagsTask) error {
		if cleaner == nil {
			return fmt.Errorf("orphan tags cleaner not configured")
		}

		deleted, err := cleaner.DeleteOrphanTags()
		if err != nil {
			return fmt.Errorf("cleanup orphan tags: %w", err)
		}

		log.Printf("[TASK] Cleaned up %d orphan tags", deleted)
		return nil
	}
}

// NewCleanupOrphanTagsQueue creates a backlite queue for tag cleanup tasks.
func NewCleanupOrphanTagsQueue(cleaner OrphanTagsCleaner) backlite.Queue {
	return backlite.NewQueue(CleanupOrphanTagsProcessor(cleaner))
}
