package tasks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mikestefanello/backlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := DefaultConfig()
	cfg.Workers = 1

	client, err := NewClient(dbPath, cfg)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify tasks database was created
	tasksDBPath := filepath.Join(tmpDir, "test-tasks.db")
	_, err = os.Stat(tasksDBPath)
	assert.NoError(t, err, "tasks database should be created")

	err = client.Close()
	assert.NoError(t, err)
}

func TestClientStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := DefaultConfig()
	cfg.Workers = 1

	client, err := NewClient(dbPath, cfg)
	require.NoError(t, err)
	defer client.Close()

	// Start client in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Start(ctx)

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should complete successfully
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	success := client.Stop(stopCtx)
	assert.True(t, success, "stop should succeed gracefully")
}

// TestTask is a simple task for testing
type TestTask struct {
	Value string `json:"value"`
}

func (t TestTask) Config() backlite.QueueConfig {
	return backlite.QueueConfig{
		Name:        "test_task",
		MaxAttempts: 1,
		Backoff:     time.Second,
		Timeout:     5 * time.Second,
	}
}

func TestTaskEnqueue(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := DefaultConfig()
	cfg.Workers = 1

	client, err := NewClient(dbPath, cfg)
	require.NoError(t, err)
	defer client.Close()

	// Create and register a test queue
	executed := make(chan string, 1)
	queue := backlite.NewQueue(func(ctx context.Context, task TestTask) error {
		executed <- task.Value
		return nil
	})
	client.Register(queue)

	// Start client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Start(ctx)

	// Enqueue a task
	ids, err := client.Add(TestTask{Value: "hello"}).Save()
	require.NoError(t, err)
	assert.Len(t, ids, 1)

	// Wait for task to be executed
	select {
	case val := <-executed:
		assert.Equal(t, "hello", val)
	case <-time.After(5 * time.Second):
		t.Fatal("task was not executed within timeout")
	}
}

func TestEnrichBookTaskConfig(t *testing.T) {
	task := EnrichBookTask{BookID: 123}
	cfg := task.Config()

	assert.Equal(t, "enrich_book", cfg.Name)
	assert.Equal(t, 3, cfg.MaxAttempts)
	assert.Equal(t, 30*time.Second, cfg.Backoff)
	assert.Equal(t, 2*time.Minute, cfg.Timeout)
	assert.NotNil(t, cfg.Retention)
}

func TestEnrichAllBooksTaskConfig(t *testing.T) {
	task := EnrichAllBooksTask{UserID: 1}
	cfg := task.Config()

	assert.Equal(t, "enrich_all_books", cfg.Name)
	assert.Equal(t, 1, cfg.MaxAttempts)
	assert.Equal(t, 60*time.Minute, cfg.Timeout)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 2, cfg.Workers)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, time.Minute, cfg.RetryDelay)
	assert.Equal(t, 5*time.Minute, cfg.TaskTimeout)
	assert.Equal(t, 15*time.Minute, cfg.ReleaseAfter)
	assert.Equal(t, time.Hour, cfg.CleanupInterval)
	assert.Equal(t, 24*time.Hour, cfg.RetentionDuration)
}
