// Package scheduler manages cron-based background sync operations.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/robfig/cron/v3"
)

// ObsidianSyncDataSource provides data needed for Obsidian exports.
type ObsidianSyncDataSource interface {
	GetAllBooks() ([]entities.Book, error)
}

// VocabularyReader provides vocabulary data for Obsidian exports.
type VocabularyReader interface {
	GetAllWords(userID uint, limit, offset int) ([]entities.Word, int64, error)
}

// ObsidianSyncScheduler manages periodic exports to Obsidian vault
type ObsidianSyncScheduler struct {
	books         ObsidianSyncDataSource
	vocabulary    VocabularyReader
	settingsStore *settingsstore.SettingsStore
	auditService  *audit.Service

	cron       *cron.Cron
	entryID    cron.EntryID
	mu         sync.RWMutex
	isRunning  bool
	cancelFunc context.CancelFunc
}

// NewObsidianSyncScheduler creates a new scheduler instance
func NewObsidianSyncScheduler(books ObsidianSyncDataSource, vocabulary VocabularyReader, settingsStore *settingsstore.SettingsStore, auditService *audit.Service) *ObsidianSyncScheduler {
	return &ObsidianSyncScheduler{
		books:         books,
		vocabulary:    vocabulary,
		settingsStore: settingsStore,
		auditService:  auditService,
		cron:          cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow))),
	}
}

// Start begins the scheduler if sync is enabled
func (s *ObsidianSyncScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}

	config := s.settingsStore.GetObsidianSyncConfig()

	if !config.Enabled {
		log.Printf("Obsidian sync scheduler: disabled")
		return nil
	}

	if config.ExportDir == "" {
		log.Printf("Obsidian sync scheduler: export directory not configured, skipping")
		return nil
	}

	if err := settingsstore.ValidateCronSchedule(config.Schedule); err != nil {
		return fmt.Errorf("invalid cron schedule '%s': %w", config.Schedule, err)
	}

	entryID, err := s.cron.AddFunc(config.Schedule, func() {
		s.runSync()
	})
	if err != nil {
		return fmt.Errorf("failed to schedule sync job: %w", err)
	}
	s.entryID = entryID

	var cancelCtx context.Context
	cancelCtx, s.cancelFunc = context.WithCancel(ctx)

	s.cron.Start()
	s.isRunning = true

	nextRun, _ := settingsstore.GetNextRunTime(config.Schedule)
	log.Printf("Obsidian sync scheduler: started with schedule '%s' (%s). Next run: %v",
		config.Schedule,
		settingsstore.GetCronDescription(config.Schedule),
		nextRun)

	go func() {
		<-cancelCtx.Done()
		s.Stop()
	}()

	return nil
}

// Stop gracefully stops the scheduler
func (s *ObsidianSyncScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.isRunning = false
	s.cancelFunc = nil

	log.Printf("Obsidian sync scheduler: stopped")
}

// Reschedule updates the schedule (call after settings change)
func (s *ObsidianSyncScheduler) Reschedule() error {
	s.mu.Lock()
	wasRunning := s.isRunning
	s.mu.Unlock()

	if wasRunning {
		s.Stop()
	}

	return s.Start(context.Background())
}

// RunNow triggers an immediate sync
func (s *ObsidianSyncScheduler) RunNow() error {
	go s.runSync()
	return nil
}

// IsRunning returns whether the scheduler is active
func (s *ObsidianSyncScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetNextRunTime returns when the next sync will occur
func (s *ObsidianSyncScheduler) GetNextRunTime() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isRunning {
		return nil
	}

	entries := s.cron.Entries()
	for _, entry := range entries {
		if entry.ID == s.entryID {
			t := entry.Next
			return &t
		}
	}
	return nil
}

// runSync performs the actual sync operation
func (s *ObsidianSyncScheduler) runSync() {
	config := s.settingsStore.GetObsidianSyncConfig()

	if !config.Enabled {
		log.Printf("Obsidian sync: skipped (disabled)")
		return
	}

	if config.ExportDir == "" {
		log.Printf("Obsidian sync: skipped (export directory not configured)")
		_ = s.settingsStore.SetObsidianSyncStatus("failed", "Export directory not configured")
		s.logAudit("Export directory not configured", fmt.Errorf("export directory not configured"))
		return
	}

	log.Printf("Obsidian sync: starting export to %s", config.ExportDir)
	startTime := time.Now()

	books, err := s.books.GetAllBooks()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get books from database: %v", err)
		log.Printf("Obsidian sync: %s", errMsg)
		_ = s.settingsStore.SetObsidianSyncStatus("failed", errMsg)
		s.logAudit(errMsg, err)
		return
	}

	if len(books) == 0 {
		log.Printf("Obsidian sync: no books to export")
		_ = s.settingsStore.SetObsidianSyncStatus("success", "No books to export")
		s.logAudit("No books to export", nil)
		return
	}

	exporter := exporters.NewMarkdownExporter(config.ExportDir)
	result, err := exporter.Export(books)
	if err != nil {
		errMsg := fmt.Sprintf("Export failed: %v", err)
		log.Printf("Obsidian sync: %s", errMsg)
		_ = s.settingsStore.SetObsidianSyncStatus("failed", errMsg)
		s.logAudit(errMsg, err)
		return
	}

	// Export vocabulary words
	var wordCount int
	if s.vocabulary != nil {
		words, _, vocabErr := s.vocabulary.GetAllWords(0, 0, 0)
		if vocabErr != nil {
			log.Printf("Obsidian sync: warning - failed to get vocabulary words: %v", vocabErr)
		} else if len(words) > 0 {
			if exportErr := exporter.ExportVocabulary(words); exportErr != nil {
				log.Printf("Obsidian sync: warning - failed to export vocabulary: %v", exportErr)
			} else {
				wordCount = len(words)
			}
		}
	}

	duration := time.Since(startTime)
	successMsg := fmt.Sprintf("Exported %d books, %d highlights, %d vocabulary words in %v",
		result.BooksProcessed, result.HighlightsProcessed, wordCount, duration.Round(time.Millisecond))
	log.Printf("Obsidian sync: %s", successMsg)
	_ = s.settingsStore.SetObsidianSyncStatus("success", successMsg)
	s.logAudit(successMsg, nil)
}

func (s *ObsidianSyncScheduler) logAudit(description string, err error) {
	if s.auditService == nil {
		return
	}
	s.auditService.LogSync(0, "obsidian_sync", description, err)
}
