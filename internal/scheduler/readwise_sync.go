package scheduler

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/readwise"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/robfig/cron/v3"
)

// ReadwiseSyncScheduler manages periodic imports from Readwise API
type ReadwiseSyncScheduler struct {
	db            *database.Database
	settingsStore *settingsstore.SettingsStore
	client        *readwise.Client
	auditService  *audit.Service

	cron       *cron.Cron
	entryID    cron.EntryID
	mu         sync.RWMutex
	isRunning  bool
	isSyncing  bool
	cancelFunc context.CancelFunc
}

// NewReadwiseSyncScheduler creates a new scheduler instance
func NewReadwiseSyncScheduler(db *database.Database, settingsStore *settingsstore.SettingsStore, client *readwise.Client, auditService *audit.Service) *ReadwiseSyncScheduler {
	return &ReadwiseSyncScheduler{
		db:            db,
		settingsStore: settingsStore,
		client:        client,
		auditService:  auditService,
		cron:          cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow))),
	}
}

// Start begins the scheduler if sync is enabled
func (s *ReadwiseSyncScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}

	config := s.settingsStore.GetReadwiseSyncConfig()

	if !config.Enabled {
		log.Printf("Readwise sync scheduler: disabled")
		return nil
	}

	if config.Token == "" {
		log.Printf("Readwise sync scheduler: token not configured, skipping")
		return nil
	}

	// Validate schedule
	if err := settingsstore.ValidateCronSchedule(config.Schedule); err != nil {
		return fmt.Errorf("invalid cron schedule '%s': %w", config.Schedule, err)
	}

	// Add the sync job
	entryID, err := s.cron.AddFunc(config.Schedule, func() {
		s.runSync()
	})
	if err != nil {
		return fmt.Errorf("failed to schedule sync job: %w", err)
	}
	s.entryID = entryID

	// Create cancellable context
	var cancelCtx context.Context
	cancelCtx, s.cancelFunc = context.WithCancel(ctx)

	// Start cron scheduler
	s.cron.Start()
	s.isRunning = true

	// Calculate next run
	nextRun, _ := settingsstore.GetNextRunTime(config.Schedule)
	log.Printf("Readwise sync scheduler: started with schedule '%s' (%s). Next run: %v",
		config.Schedule,
		settingsstore.GetCronDescription(config.Schedule),
		nextRun)

	// Monitor for context cancellation
	go func() {
		<-cancelCtx.Done()
		s.Stop()
	}()

	return nil
}

// Stop gracefully stops the scheduler
func (s *ReadwiseSyncScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	// Stop accepting new jobs and wait for running jobs to complete
	ctx := s.cron.Stop()
	<-ctx.Done()

	s.isRunning = false
	s.cancelFunc = nil

	log.Printf("Readwise sync scheduler: stopped")
}

// Reschedule updates the schedule (call after settings change)
func (s *ReadwiseSyncScheduler) Reschedule() error {
	s.mu.Lock()
	wasRunning := s.isRunning
	s.mu.Unlock()

	if wasRunning {
		s.Stop()
	}

	// Restart with new settings
	return s.Start(context.Background())
}

// RunNow triggers an immediate sync
func (s *ReadwiseSyncScheduler) RunNow() error {
	go s.runSync()
	return nil
}

// IsRunning returns whether the scheduler is active
func (s *ReadwiseSyncScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// IsSyncing returns whether a sync is currently in progress
func (s *ReadwiseSyncScheduler) IsSyncing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isSyncing
}

// GetNextRunTime returns when the next sync will occur
func (s *ReadwiseSyncScheduler) GetNextRunTime() *time.Time {
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
func (s *ReadwiseSyncScheduler) runSync() {
	s.mu.Lock()
	if s.isSyncing {
		s.mu.Unlock()
		log.Printf("Readwise sync: skipped (already syncing)")
		return
	}
	s.isSyncing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isSyncing = false
		s.mu.Unlock()
	}()

	config := s.settingsStore.GetReadwiseSyncConfig()

	if !config.Enabled {
		log.Printf("Readwise sync: skipped (disabled)")
		return
	}

	if config.Token == "" {
		log.Printf("Readwise sync: skipped (token not configured)")
		_ = s.settingsStore.SetReadwiseSyncStatus("failed", "Token not configured", 0)
		s.logAudit("readwise_sync", "Token not configured", fmt.Errorf("token not configured"))
		return
	}

	log.Printf("Readwise sync: starting import from Readwise API")
	startTime := time.Now()

	// Get last sync time for incremental sync
	lastSyncAt := s.settingsStore.GetReadwiseSyncLastAt()
	if lastSyncAt != nil {
		log.Printf("Readwise sync: incremental sync from %s", lastSyncAt.Format(time.RFC3339))
	} else {
		log.Printf("Readwise sync: full sync (no previous sync found)")
	}

	// Fetch data from Readwise API
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	books, err := s.client.ExportAll(ctx, config.Token, lastSyncAt)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to fetch from Readwise API: %v", err)
		log.Printf("Readwise sync: %s", errMsg)
		_ = s.settingsStore.SetReadwiseSyncStatus("failed", errMsg, 0)
		s.logAudit("readwise_sync", errMsg, err)
		return
	}

	if len(books) == 0 {
		log.Printf("Readwise sync: no new books/highlights to import")
		_ = s.settingsStore.SetReadwiseSyncStatus("success", "No new data to import", 0)
		s.logAudit("readwise_sync", "No new data to import", nil)
		return
	}

	// Get or create the Readwise source
	source, err := s.db.GetSourceByName("readwise")
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get readwise source: %v", err)
		log.Printf("Readwise sync: %s", errMsg)
		_ = s.settingsStore.SetReadwiseSyncStatus("failed", errMsg, 0)
		s.logAudit("readwise_sync", errMsg, err)
		return
	}

	// Convert and save books
	var totalHighlights int
	var booksProcessed int
	for _, bookData := range books {
		book := convertReadwiseBook(bookData, source.ID)
		totalHighlights += len(book.Highlights)

		if err := s.db.SaveBook(&book); err != nil {
			log.Printf("Readwise sync: warning - failed to save book '%s': %v", book.Title, err)
			continue
		}
		booksProcessed++
	}

	duration := time.Since(startTime)
	successMsg := fmt.Sprintf("Imported %d books with %d highlights in %v",
		booksProcessed, totalHighlights, duration.Round(time.Millisecond))
	log.Printf("Readwise sync: %s", successMsg)
	_ = s.settingsStore.SetReadwiseSyncStatus("success", successMsg, totalHighlights)
	s.logAudit("readwise_sync", successMsg, nil)
}

func (s *ReadwiseSyncScheduler) logAudit(action, description string, err error) {
	if s.auditService == nil {
		return
	}
	s.auditService.LogSync(0, action, description, err)
}

// convertReadwiseBook converts Readwise API book data to our Book entity
func convertReadwiseBook(data readwise.BookData, sourceID uint) entities.Book {
	book := entities.Book{
		Title:      data.Title,
		Author:     data.Author,
		CoverURL:   data.CoverImageURL,
		ASIN:       data.ASIN,
		ExternalID: strconv.Itoa(data.UserBookID),
		SourceID:   sourceID,
	}

	for _, h := range data.Highlights {
		highlight := convertReadwiseHighlight(h, sourceID)
		book.Highlights = append(book.Highlights, highlight)
	}

	return book
}

// convertReadwiseHighlight converts Readwise API highlight data to our Highlight entity
func convertReadwiseHighlight(data readwise.HighlightData, sourceID uint) entities.Highlight {
	locationType := mapLocationTypeFromReadwise(data.LocationType)

	highlight := entities.Highlight{
		Text:          data.Text,
		Note:          data.Note,
		LocationType:  locationType,
		LocationValue: data.Location,
		LocationEnd:   data.EndLocation,
		Color:         data.Color,
		HighlightedAt: data.HighlightedAt,
		IsFavorite:    data.IsFavorite,
		IsDiscarded:   data.IsDiscarded,
		ExternalID:    strconv.Itoa(data.ID),
		SourceID:      sourceID,
	}

	return highlight
}

// mapLocationTypeFromReadwise maps Readwise location types to our enum
func mapLocationTypeFromReadwise(locationType string) entities.LocationType {
	switch locationType {
	case "page":
		return entities.LocationTypePage
	case "location":
		return entities.LocationTypeLocation
	case "time_offset":
		return entities.LocationTypeTime
	case "order":
		return entities.LocationTypePosition
	default:
		return entities.LocationTypeNone
	}
}
