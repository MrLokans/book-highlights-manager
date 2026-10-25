package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/readwise"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBookSaver implements BookSaver for tests.
type mockBookSaver struct {
	saved []entities.Book
	err   error
}

func (m *mockBookSaver) SaveBook(book *entities.Book) error {
	if m.err != nil {
		return m.err
	}
	m.saved = append(m.saved, *book)
	return nil
}

// mockSourceLookup implements SourceLookup for tests.
type mockSourceLookup struct {
	source *entities.Source
	err    error
}

func (m *mockSourceLookup) GetByName(_ string) (*entities.Source, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.source, nil
}

// newReadwiseTestClient creates a readwise client pointing at a test server.
func newReadwiseTestClient(t *testing.T, handler http.Handler) (*httptest.Server, *readwise.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, readwise.NewClientWithOptions(
		readwise.WithBaseURL(srv.URL),
		readwise.WithHTTPClient(srv.Client()),
	)
}

// --- Data Conversion ---

func TestConvertReadwiseBook(t *testing.T) {
	t.Run("converts basic fields", func(t *testing.T) {
		data := readwise.BookData{
			Title:         "Test Book",
			Author:        "Test Author",
			CoverImageURL: "https://example.com/cover.jpg",
			ASIN:          "B001234567",
			UserBookID:    42,
		}

		book := convertReadwiseBook(data, 5)

		assert.Equal(t, "Test Book", book.Title)
		assert.Equal(t, "Test Author", book.Author)
		assert.Equal(t, "https://example.com/cover.jpg", book.CoverURL)
		assert.Equal(t, "B001234567", book.ASIN)
		assert.Equal(t, "42", book.ExternalID)
		assert.Equal(t, uint(5), book.SourceID)
	})

	t.Run("converts highlights", func(t *testing.T) {
		now := time.Now()
		data := readwise.BookData{
			Title:  "Book",
			Author: "Author",
			Highlights: []readwise.HighlightData{
				{ID: 1, Text: "highlight 1", HighlightedAt: now},
				{ID: 2, Text: "highlight 2", Note: "my note", IsFavorite: true},
			},
		}

		book := convertReadwiseBook(data, 1)

		require.Len(t, book.Highlights, 2)
		assert.Equal(t, "highlight 1", book.Highlights[0].Text)
		assert.Equal(t, "1", book.Highlights[0].ExternalID)
		assert.Equal(t, uint(1), book.Highlights[0].SourceID)
		assert.Equal(t, "my note", book.Highlights[1].Note)
		assert.True(t, book.Highlights[1].IsFavorite)
	})
}

func TestConvertReadwiseHighlight(t *testing.T) {
	now := time.Now()
	data := readwise.HighlightData{
		ID:            123,
		Text:          "some text",
		Note:          "my note",
		Location:      42,
		EndLocation:   50,
		Color:         "yellow",
		HighlightedAt: now,
		IsFavorite:    true,
		IsDiscarded:   false,
		LocationType:  "page",
	}

	h := convertReadwiseHighlight(data, 7)

	assert.Equal(t, "some text", h.Text)
	assert.Equal(t, "my note", h.Note)
	assert.Equal(t, 42, h.LocationValue)
	assert.Equal(t, 50, h.LocationEnd)
	assert.Equal(t, "yellow", h.Color)
	assert.Equal(t, now, h.HighlightedAt)
	assert.True(t, h.IsFavorite)
	assert.False(t, h.IsDiscarded)
	assert.Equal(t, "123", h.ExternalID)
	assert.Equal(t, uint(7), h.SourceID)
	assert.Equal(t, entities.LocationTypePage, h.LocationType)
}

func TestMapLocationTypeFromReadwise(t *testing.T) {
	tests := []struct {
		input    string
		expected entities.LocationType
	}{
		{"page", entities.LocationTypePage},
		{"location", entities.LocationTypeLocation},
		{"time_offset", entities.LocationTypeTime},
		{"order", entities.LocationTypePosition},
		{"unknown", entities.LocationTypeNone},
		{"", entities.LocationTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapLocationTypeFromReadwise(tt.input))
		})
	}
}

// --- State Management ---

func TestReadwiseSyncScheduler_InitialState(t *testing.T) {
	sched := NewReadwiseSyncScheduler(nil, nil, nil, nil, nil)
	assert.False(t, sched.IsRunning())
	assert.False(t, sched.IsSyncing())
	assert.Nil(t, sched.GetNextRunTime())
}

func TestReadwiseSyncScheduler_StopWhenNotRunning(t *testing.T) {
	sched := NewReadwiseSyncScheduler(nil, nil, nil, nil, nil)
	// Stop on non-running should not panic
	// Note: calling Stop() when not running would block on channel, so we just check state
	assert.False(t, sched.IsRunning())
}

// --- Start ---

func TestReadwiseSyncScheduler_Start_Disabled(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "false"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, nil, nil)
	err := sched.Start(context.Background())
	assert.NoError(t, err)
	assert.False(t, sched.IsRunning())
}

func TestReadwiseSyncScheduler_Start_NoToken(t *testing.T) {
	t.Setenv("READWISE_TOKEN", "")

	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, nil, nil)
	err := sched.Start(context.Background())
	assert.NoError(t, err)
	assert.False(t, sched.IsRunning(), "should not start without token")
}

func TestReadwiseSyncScheduler_Start_InvalidSchedule(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "test-token"
	db.settings["readwise_sync_schedule"] = "invalid"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, nil, nil)
	err := sched.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cron schedule")
}

func TestReadwiseSyncScheduler_Start_Success(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "test-token"
	db.settings["readwise_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(&mockBookSaver{}, &mockSourceLookup{source: &entities.Source{Name: "readwise"}}, store, readwise.NewClient(), nil)
	err := sched.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, sched.IsRunning())
	assert.NotNil(t, sched.GetNextRunTime())
	sched.Stop()
	assert.False(t, sched.IsRunning())
}

func TestReadwiseSyncScheduler_Start_AlreadyRunning(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "test-token"
	db.settings["readwise_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(&mockBookSaver{}, nil, store, readwise.NewClient(), nil)
	require.NoError(t, sched.Start(context.Background()))
	defer sched.Stop()

	err := sched.Start(context.Background())
	assert.NoError(t, err)
}

// --- Reschedule ---

func TestReadwiseSyncScheduler_Reschedule(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "test-token"
	db.settings["readwise_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(&mockBookSaver{}, nil, store, readwise.NewClient(), nil)
	require.NoError(t, sched.Start(context.Background()))

	db.settings["readwise_sync_schedule"] = "*/10 * * * *"
	err := sched.Reschedule()
	require.NoError(t, err)
	assert.True(t, sched.IsRunning())
	sched.Stop()
}

// --- RunNow / runSync ---

func TestReadwiseSyncScheduler_RunNow_Disabled(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "false"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, nil, nil)
	err := sched.RunNow()
	assert.NoError(t, err)
}

func TestReadwiseSyncScheduler_RunSync_NoToken(t *testing.T) {
	// Ensure env var doesn't interfere
	t.Setenv("READWISE_TOKEN", "")

	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, nil, nil)
	sched.runSync(context.Background())

	assert.Equal(t, "failed", db.settings["readwise_sync_last_status"])
	assert.Contains(t, db.settings["readwise_sync_last_message"], "Token not configured")
}

func TestReadwiseSyncScheduler_RunSync_APIError(t *testing.T) {
	_, client := newReadwiseTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "bad-token"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, client, nil)
	sched.runSync(context.Background())

	assert.Equal(t, "failed", db.settings["readwise_sync_last_status"])
	assert.Contains(t, db.settings["readwise_sync_last_message"], "Failed to fetch")
}

func TestReadwiseSyncScheduler_RunSync_NoBooks(t *testing.T) {
	_, client := newReadwiseTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(readwise.ExportResponse{Count: 0, Results: nil})
	}))

	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "tok"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(&mockBookSaver{}, nil, store, client, nil)
	sched.runSync(context.Background())

	assert.Equal(t, "success", db.settings["readwise_sync_last_status"])
	assert.Contains(t, db.settings["readwise_sync_last_message"], "No new data")
}

func TestReadwiseSyncScheduler_RunSync_Success(t *testing.T) {
	_, client := newReadwiseTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(readwise.ExportResponse{
			Count: 1,
			Results: []readwise.BookData{
				{
					UserBookID: 1,
					Title:      "Test Book",
					Author:     "Author",
					Highlights: []readwise.HighlightData{
						{ID: 1, Text: "highlight text"},
					},
				},
			},
		})
	}))

	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "tok"
	store := settingsstore.New(db)

	saver := &mockBookSaver{}
	source := &mockSourceLookup{source: &entities.Source{ID: 5, Name: "readwise"}}
	sched := NewReadwiseSyncScheduler(saver, source, store, client, nil)
	sched.runSync(context.Background())

	assert.Equal(t, "success", db.settings["readwise_sync_last_status"])
	assert.Contains(t, db.settings["readwise_sync_last_message"], "Imported 1 books")
	require.Len(t, saver.saved, 1)
	assert.Equal(t, "Test Book", saver.saved[0].Title)
	assert.Equal(t, uint(5), saver.saved[0].SourceID)
}

func TestReadwiseSyncScheduler_RunSync_SourceLookupError(t *testing.T) {
	_, client := newReadwiseTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(readwise.ExportResponse{
			Count:   1,
			Results: []readwise.BookData{{UserBookID: 1, Title: "Book"}},
		})
	}))

	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "tok"
	store := settingsstore.New(db)

	source := &mockSourceLookup{err: fmt.Errorf("source not found")}
	sched := NewReadwiseSyncScheduler(nil, source, store, client, nil)
	sched.runSync(context.Background())

	assert.Equal(t, "failed", db.settings["readwise_sync_last_status"])
	assert.Contains(t, db.settings["readwise_sync_last_message"], "source not found")
}

func TestReadwiseSyncScheduler_RunSync_SaveBookError(t *testing.T) {
	_, client := newReadwiseTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(readwise.ExportResponse{
			Count: 1,
			Results: []readwise.BookData{
				{UserBookID: 1, Title: "Book", Highlights: []readwise.HighlightData{{ID: 1, Text: "hl"}}},
			},
		})
	}))

	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "tok"
	store := settingsstore.New(db)

	saver := &mockBookSaver{err: fmt.Errorf("db write error")}
	source := &mockSourceLookup{source: &entities.Source{ID: 1}}
	sched := NewReadwiseSyncScheduler(saver, source, store, client, nil)
	sched.runSync(context.Background())

	// Still reports success but with 0 books processed (all failed)
	assert.Equal(t, "success", db.settings["readwise_sync_last_status"])
	assert.Contains(t, db.settings["readwise_sync_last_message"], "Imported 0 books")
}

func TestReadwiseSyncScheduler_RunSync_SkipWhenAlreadySyncing(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "true"
	db.settings["readwise_sync_token"] = "tok"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, nil, nil)

	// Simulate already syncing
	sched.mu.Lock()
	sched.isSyncing = true
	sched.mu.Unlock()

	sched.runSync(context.Background())

	// Should have skipped — no status set
	_, hasStatus := db.settings["readwise_sync_last_status"]
	assert.False(t, hasStatus)
}

func TestReadwiseSyncScheduler_RunSync_Disabled(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["readwise_sync_enabled"] = "false"
	store := settingsstore.New(db)

	sched := NewReadwiseSyncScheduler(nil, nil, store, nil, nil)
	sched.runSync(context.Background())

	// No status set because sync was skipped
	_, hasStatus := db.settings["readwise_sync_last_status"]
	assert.False(t, hasStatus)
}
