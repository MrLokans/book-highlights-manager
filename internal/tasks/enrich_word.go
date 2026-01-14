package tasks

import (
	"context"
	"fmt"
	"log"
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
				log.Printf("[TASK] Failed to update word status: %v", updateErr)
			}
			return fmt.Errorf("lookup word %q: %w", word.Word, err)
		}

		if err := store.SaveDefinitions(task.WordID, result.Definitions); err != nil {
			return fmt.Errorf("save definitions for word %d: %w", task.WordID, err)
		}

		if err := store.UpdateWordStatus(task.WordID, entities.WordStatusEnriched, ""); err != nil {
			return fmt.Errorf("update word status: %w", err)
		}

		log.Printf("[TASK] Enriched word %q with %d definitions", word.Word, len(result.Definitions))
		return nil
	}
}

func NewEnrichWordQueue(store WordEnricher, dictClient dictionary.Client) backlite.Queue {
	return backlite.NewQueue(EnrichWordProcessor(store, dictClient))
}

// EnrichAllPendingWordsTask enriches all words with pending status.
type EnrichAllPendingWordsTask struct{}

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

func EnrichAllPendingWordsProcessor(store WordEnricher, dictClient dictionary.Client) backlite.QueueProcessor[EnrichAllPendingWordsTask] {
	return func(ctx context.Context, task EnrichAllPendingWordsTask) error {
		words, err := store.GetPendingWords(0) // 0 = no limit
		if err != nil {
			return fmt.Errorf("get pending words: %w", err)
		}

		var enriched, failed int
		for _, word := range words {
			select {
			case <-ctx.Done():
				log.Printf("[TASK] Context cancelled, enriched %d words, %d failed", enriched, failed)
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

		log.Printf("[TASK] Enriched %d words, %d failed out of %d total", enriched, failed, len(words))
		return nil
	}
}

func NewEnrichAllPendingWordsQueue(store WordEnricher, dictClient dictionary.Client) backlite.Queue {
	return backlite.NewQueue(EnrichAllPendingWordsProcessor(store, dictClient))
}
