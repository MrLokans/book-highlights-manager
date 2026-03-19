package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Cleanup Tags ---

type mockTagsCleaner struct {
	deleted int64
	err     error
}

func (m *mockTagsCleaner) DeleteOrphanTags() (int64, error) {
	return m.deleted, m.err
}

func TestCleanupOrphanTagsTask_Config(t *testing.T) {
	task := CleanupOrphanTagsTask{}
	cfg := task.Config()
	assert.Equal(t, "cleanup_orphan_tags", cfg.Name)
	assert.Equal(t, 1, cfg.MaxAttempts)
}

func TestCleanupOrphanTagsProcessor(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		processor := CleanupOrphanTagsProcessor(&mockTagsCleaner{deleted: 3})
		err := processor(context.Background(), CleanupOrphanTagsTask{})
		assert.NoError(t, err)
	})

	t.Run("returns error on failure", func(t *testing.T) {
		processor := CleanupOrphanTagsProcessor(&mockTagsCleaner{err: errors.New("db error")})
		err := processor(context.Background(), CleanupOrphanTagsTask{})
		assert.Error(t, err)
	})

	t.Run("returns error when nil cleaner", func(t *testing.T) {
		processor := CleanupOrphanTagsProcessor(nil)
		err := processor(context.Background(), CleanupOrphanTagsTask{})
		assert.Error(t, err)
	})
}

// --- Cleanup Audit ---

func TestCleanupAuditEventsTask_Config(t *testing.T) {
	task := CleanupAuditEventsTask{}
	cfg := task.Config()
	assert.Equal(t, "cleanup_audit_events", cfg.Name)
}

// --- Enrich Word ---

func TestEnrichWordTask_Config(t *testing.T) {
	task := EnrichWordTask{WordID: 42}
	cfg := task.Config()
	assert.Equal(t, "enrich_word", cfg.Name)
	assert.Equal(t, 42, int(task.WordID))
}

func TestEnrichAllPendingWordsTask_Config(t *testing.T) {
	task := EnrichAllPendingWordsTask{}
	cfg := task.Config()
	assert.Equal(t, "enrich_all_words", cfg.Name)
}

// --- Enrich Book ---

func TestEnrichBookTask_Config(t *testing.T) {
	task := EnrichBookTask{BookID: 1}
	cfg := task.Config()
	assert.Equal(t, "enrich_book", cfg.Name)
}

func TestEnrichAllBooksTask_Config(t *testing.T) {
	task := EnrichAllBooksTask{}
	cfg := task.Config()
	assert.Equal(t, "enrich_all_books", cfg.Name)
}

// --- Queue constructors ---

func TestNewCleanupOrphanTagsQueue(t *testing.T) {
	q := NewCleanupOrphanTagsQueue(&mockTagsCleaner{})
	require.NotNil(t, q)
}
