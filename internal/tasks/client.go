package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mikestefanello/backlite"
)

// Client wraps backlite to provide task queue functionality.
type Client struct {
	client *backlite.Client
	db     *sql.DB
	config Config

	mu      sync.RWMutex
	started bool
}

// NewClient creates a new task queue client with a dedicated SQLite database.
// The database is stored alongside the main database with a "-tasks" suffix.
func NewClient(mainDBPath string, cfg Config) (*Client, error) {
	// Create tasks database path alongside main DB
	dir := filepath.Dir(mainDBPath)
	base := filepath.Base(mainDBPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	tasksDBPath := filepath.Join(dir, name+"-tasks"+ext)

	// Open dedicated SQLite connection for tasks with WAL mode
	db, err := sql.Open("sqlite3", tasksDBPath+"?_journal=WAL&_timeout=5000&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open tasks database: %w", err)
	}

	// Configure connection pool for concurrent workers
	db.SetMaxOpenConns(cfg.Workers + 5)
	db.SetMaxIdleConns(cfg.Workers + 2)
	db.SetConnMaxLifetime(time.Hour)

	// Create backlite client
	client, err := backlite.NewClient(backlite.ClientConfig{
		DB:              db,
		NumWorkers:      cfg.Workers,
		ReleaseAfter:    cfg.ReleaseAfter,
		CleanupInterval: cfg.CleanupInterval,
		Logger:          &stdLogger{},
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create backlite client: %w", err)
	}

	// Install schema
	if err := client.Install(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to install backlite schema: %w", err)
	}

	return &Client{
		client: client,
		db:     db,
		config: cfg,
	}, nil
}

// Register registers task queues with the client.
// Must be called before Start().
func (c *Client) Register(queues ...backlite.Queue) {
	for _, q := range queues {
		c.client.Register(q)
	}
}

// Start begins processing tasks. This is non-blocking and should be called
// in a goroutine. Use Stop() for graceful shutdown.
func (c *Client) Start(ctx context.Context) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.started = true
	c.mu.Unlock()

	log.Printf("Task queue started with %d workers", c.config.Workers)
	c.client.Start(ctx)
}

// Stop gracefully shuts down the task queue, waiting for active tasks to complete.
// Returns true if all workers finished before the context deadline.
func (c *Client) Stop(ctx context.Context) bool {
	c.mu.RLock()
	if !c.started {
		c.mu.RUnlock()
		return true
	}
	c.mu.RUnlock()

	log.Println("Stopping task queue...")
	success := c.client.Stop(ctx)
	if success {
		log.Println("Task queue stopped gracefully")
	} else {
		log.Println("Task queue stopped with timeout (some tasks may not have completed)")
	}
	return success
}

// Close releases all resources. Should be called after Stop().
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Add starts an operation to enqueue one or more tasks.
func (c *Client) Add(tasks ...backlite.Task) *backlite.TaskAddOp {
	return c.client.Add(tasks...)
}

// Status returns the status of a task by ID.
func (c *Client) Status(ctx context.Context, taskID string) (backlite.TaskStatus, error) {
	return c.client.Status(ctx, taskID)
}

// DB returns the underlying database connection for use with backlite UI.
func (c *Client) DB() *sql.DB {
	return c.db
}

// Client returns the underlying backlite client for advanced operations.
func (c *Client) Client() *backlite.Client {
	return c.client
}

// stdLogger implements backlite.Logger using standard library log.
type stdLogger struct{}

func (l *stdLogger) Info(message string, params ...any) {
	log.Printf("[TASK] "+message, params...)
}

func (l *stdLogger) Error(message string, params ...any) {
	log.Printf("[TASK ERROR] "+message, params...)
}
