package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mikestefanello/backlite"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/entities"
)

// WordEnricher defines the interface for word enrichment operations.
type WordEnricher interface {
	GetWordByID(id uint) (*entities.Word, error)
	SaveDefinitions(wordID uint, definitions []entities.WordDefinition) error
	UpdateWordStatus(id uint, status entities.WordStatus, errorMsg string) error
	GetPendingWords(limit int) ([]entities.Word, error)
}

// EnrichWordTask enriches a single word with dictionary definitions.
type EnrichWordTask struct {
	WordID uint `json:"word_id"`
}

// Config returns the queue configuration for single-word enrichment.
func (t EnrichWordTask) Config() backlite.QueueConfig {
	return backlite.QueueConfig{
		Name:        "enrich_word",
		MaxAttempts: 3,
		Backoff:     30 * time.Second,
		Timeout:     1 * time.Minute,
		Retention: &backlite.Retention{
			Duration:   24 * time.Hour,
			OnlyFailed: false,
			Data:       &backlite.RetainData{OnlyFailed: true},
		},
	}
}

// EnrichWordProcessor creates a processor for word enrichment.
func EnrichWordProcessor(store WordEnricher, dictClient dictionary.Client) backlite.QueueProcessor[EnrichWordTask] {
	return func(ctx context.Context, task EnrichWordTask) error {
		word, err := store.GetWordByID(task.WordID)
		if err != nil {
			return fmt.Errorf("get word %d: %w", task.WordID, err)
		}

		result, err := dictClient.Lookup(ctx, word.Word)
		if err != nil {
			if updateErr := store.UpdateWordStatus(task.WordID, entities.WordStatusFailed, err.Error()); updateErr != nil {
				slog.Error("Task failed to update word status", "error", updateErr)
			}
			return fmt.Errorf("lookup word %q: %w", word.Word, err)
		}

		if err := store.SaveDefinitions(task.WordID, result.Definitions); err != nil {
			return fmt.Errorf("save definitions for word %d: %w", task.WordID, err)
		}

		if err := store.UpdateWordStatus(task.WordID, entities.WordStatusEnriched, ""); err != nil {
			return fmt.Errorf("update word status: %w", err)
		}

		slog.Info("Task enriched word", "word", word.Word, "definitions", len(result.Definitions))
		return nil
	}
}

// NewEnrichWordQueue creates a queue for enriching individual vocabulary words.
func NewEnrichWordQueue(store WordEnricher, dictClient dictionary.Client) backlite.Queue {
	return backlite.NewQueue(EnrichWordProcessor(store, dictClient))
}

// EnrichAllPendingWordsTask enriches all words with pending status.
type EnrichAllPendingWordsTask struct{}

// Config returns the queue configuration for batch word enrichment.
func (t EnrichAllPendingWordsTask) Config() backlite.QueueConfig {
	return backlite.QueueConfig{
		Name:        "enrich_all_words",
		MaxAttempts: 1,
		Backoff:     time.Minute,
		Timeout:     30 * time.Minute,
		Retention: &backlite.Retention{
			Duration:   24 * time.Hour,
			OnlyFailed: false,
			Data:       &backlite.RetainData{OnlyFailed: true},
		},
	}
}

// EnrichAllPendingWordsProcessor creates a processor that enriches all pending vocabulary words.
func EnrichAllPendingWordsProcessor(store WordEnricher, dictClient dictionary.Client) backlite.QueueProcessor[EnrichAllPendingWordsTask] {
	return func(ctx context.Context, _ EnrichAllPendingWordsTask) error {
		words, err := store.GetPendingWords(0) // 0 = no limit
		if err != nil {
			return fmt.Errorf("get pending words: %w", err)
		}

		var enriched, failed int
		for _, word := range words {
			select {
			case <-ctx.Done():
				slog.Info("Task context cancelled", "enriched", enriched, "failed", failed)
				return ctx.Err()
			default:
			}

			result, err := dictClient.Lookup(ctx, word.Word)
			if err != nil {
				_ = store.UpdateWordStatus(word.ID, entities.WordStatusFailed, err.Error())
				failed++
				continue
			}

			if err := store.SaveDefinitions(word.ID, result.Definitions); err != nil {
				_ = store.UpdateWordStatus(word.ID, entities.WordStatusFailed, err.Error())
				failed++
				continue
			}

			_ = store.UpdateWordStatus(word.ID, entities.WordStatusEnriched, "")
			enriched++
		}

		slog.Info("Task enriched words", "enriched", enriched, "failed", failed, "total", len(words))
		return nil
	}
}

// NewEnrichAllPendingWordsQueue creates a queue for enriching all pending vocabulary words.
func NewEnrichAllPendingWordsQueue(store WordEnricher, dictClient dictionary.Client) backlite.Queue {
	return backlite.NewQueue(EnrichAllPendingWordsProcessor(store, dictClient))
}
