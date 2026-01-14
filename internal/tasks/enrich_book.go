package tasks

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mikestefanello/backlite"
	"github.com/mrlokans/assistant/internal/metadata"
)

// EnrichBookTask enriches a single book's metadata from external sources.
type EnrichBookTask struct {
	BookID uint `json:"book_id"`
}

// Config returns the queue configuration for book enrichment tasks.
func (t EnrichBookTask) Config() backlite.QueueConfig {
	return backlite.QueueConfig{
		Name:        "enrich_book",
		MaxAttempts: 3,
		Backoff:     30 * time.Second,
		Timeout:     2 * time.Minute,
		Retention: &backlite.Retention{
			Duration:   24 * time.Hour,
			OnlyFailed: false,
			Data:       &backlite.RetainData{OnlyFailed: true},
		},
	}
}

// EnrichBookProcessor creates a processor function for EnrichBookTask.
// The processor needs access to the metadata enricher to perform the actual work.
func EnrichBookProcessor(enricher *metadata.Enricher) backlite.QueueProcessor[EnrichBookTask] {
	return func(ctx context.Context, task EnrichBookTask) error {
		if enricher == nil {
			return fmt.Errorf("enricher not configured")
		}

		result, err := enricher.EnrichBook(ctx, task.BookID)
		if err != nil {
			return fmt.Errorf("enrich book %d: %w", task.BookID, err)
		}

		if len(result.FieldsUpdated) > 0 {
			log.Printf("[TASK] Enriched book %d (%s): updated %v via %s",
				task.BookID, result.Book.Title, result.FieldsUpdated, result.SearchMethod)
		} else {
			log.Printf("[TASK] Book %d (%s): no metadata updates needed",
				task.BookID, result.Book.Title)
		}

		return nil
	}
}

// NewEnrichBookQueue creates a backlite queue for book enrichment tasks.
func NewEnrichBookQueue(enricher *metadata.Enricher) backlite.Queue {
	return backlite.NewQueue(EnrichBookProcessor(enricher))
}
