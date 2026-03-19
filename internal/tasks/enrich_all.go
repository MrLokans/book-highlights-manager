package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mikestefanello/backlite"
	"github.com/mrlokans/assistant/internal/metadata"
)

// EnrichAllBooksTask triggers enrichment for all books missing metadata.
// Runs enrichment sequentially to provide progress updates.
type EnrichAllBooksTask struct {
	// UserID optionally filters to a specific user's books (0 = all users)
	UserID uint `json:"user_id,omitempty"`
}

// Config returns the queue configuration for bulk enrichment tasks.
func (t EnrichAllBooksTask) Config() backlite.QueueConfig {
	return backlite.QueueConfig{
		Name:        "enrich_all_books",
		MaxAttempts: 1,
		Backoff:     time.Minute,
		Timeout:     60 * time.Minute, // Allow time to process all books
		Retention: &backlite.Retention{
			Duration:   24 * time.Hour,
			OnlyFailed: false,
			Data:       &backlite.RetainData{OnlyFailed: true},
		},
	}
}

// EnrichAllBooksProcessor creates a processor function for EnrichAllBooksTask.
// It uses the enricher's EnrichAllMissing method which handles progress tracking.
func EnrichAllBooksProcessor(enricher *metadata.Enricher) backlite.QueueProcessor[EnrichAllBooksTask] {
	return func(ctx context.Context, _ EnrichAllBooksTask) error {
		if enricher == nil {
			return fmt.Errorf("enricher not configured")
		}

		result, err := enricher.EnrichAllMissing(ctx)
		if err != nil {
			return fmt.Errorf("enrich all books: %w", err)
		}

		slog.Info("Task enrichment complete",
			"total", result.TotalBooks, "enriched", result.Enriched,
			"skipped", result.Skipped, "failed", result.Failed)

		return nil
	}
}

// NewEnrichAllBooksQueue creates a backlite queue for bulk enrichment tasks.
func NewEnrichAllBooksQueue(enricher *metadata.Enricher) backlite.Queue {
	return backlite.NewQueue(EnrichAllBooksProcessor(enricher))
}
