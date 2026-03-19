package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSettingsDB implements settingsstore.SettingsDB for tests.
type mockSettingsDB struct {
	settings map[string]string
}

func newMockSettingsDB() *mockSettingsDB {
	return &mockSettingsDB{settings: make(map[string]string)}
}

func (m *mockSettingsDB) GetSetting(key string) (*entities.Setting, error) {
	v, ok := m.settings[key]
	if !ok {
		return &entities.Setting{Key: key, Value: ""}, nil
	}
	return &entities.Setting{Key: key, Value: v}, nil
}

func (m *mockSettingsDB) SetSetting(key, value string) error {
	m.settings[key] = value
	return nil
}

func (m *mockSettingsDB) DeleteSetting(key string) error {
	delete(m.settings, key)
	return nil
}

// mockBookDataSource implements ObsidianSyncDataSource.
type mockBookDataSource struct {
	books []entities.Book
	err   error
}

func (m *mockBookDataSource) GetAllBooks() ([]entities.Book, error) {
	return m.books, m.err
}

// mockVocabularyReader implements VocabularyReader.
type mockVocabularyReader struct {
	words []entities.Word
	total int64
	err   error
}

func (m *mockVocabularyReader) GetAllWords(_ uint, _, _ int) ([]entities.Word, int64, error) {
	return m.words, m.total, m.err
}

// --- State management ---

func TestObsidianSyncScheduler_InitialState(t *testing.T) {
	sched := NewObsidianSyncScheduler(nil, nil, nil, nil)
	assert.False(t, sched.IsRunning())
	assert.Nil(t, sched.GetNextRunTime())
}

func TestObsidianSyncScheduler_StopWhenNotRunning(t *testing.T) {
	sched := NewObsidianSyncScheduler(nil, nil, nil, nil)
	sched.Stop() // should not panic
	assert.False(t, sched.IsRunning())
}

// --- Start ---

func TestObsidianSyncScheduler_Start_Disabled(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "false"
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(nil, nil, store, nil)
	err := sched.Start(context.Background())
	assert.NoError(t, err)
	assert.False(t, sched.IsRunning())
}

func TestObsidianSyncScheduler_Start_NoExportDir(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")

	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	// No export dir set
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(nil, nil, store, nil)
	err := sched.Start(context.Background())
	assert.NoError(t, err)
	assert.False(t, sched.IsRunning())
}

func TestObsidianSyncScheduler_Start_InvalidSchedule(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = "/tmp/test-vault"
	db.settings["obsidian_sync_schedule"] = "not-a-cron"
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(nil, nil, store, nil)
	err := sched.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cron schedule")
	assert.False(t, sched.IsRunning())
}

func TestObsidianSyncScheduler_Start_Success(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = t.TempDir()
	db.settings["obsidian_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(&mockBookDataSource{}, nil, store, nil)
	err := sched.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, sched.IsRunning())
	assert.NotNil(t, sched.GetNextRunTime())
	sched.Stop()
	assert.False(t, sched.IsRunning())
}

func TestObsidianSyncScheduler_Start_AlreadyRunning(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = t.TempDir()
	db.settings["obsidian_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(&mockBookDataSource{}, nil, store, nil)
	require.NoError(t, sched.Start(context.Background()))
	defer sched.Stop()

	// Second start is a no-op
	err := sched.Start(context.Background())
	assert.NoError(t, err)
	assert.True(t, sched.IsRunning())
}

// --- Stop ---

func TestObsidianSyncScheduler_StopViaContextCancel(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = t.TempDir()
	db.settings["obsidian_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	ctx, cancel := context.WithCancel(context.Background())
	sched := NewObsidianSyncScheduler(&mockBookDataSource{}, nil, store, nil)
	require.NoError(t, sched.Start(ctx))
	assert.True(t, sched.IsRunning())

	cancel()
	// Give the goroutine time to react
	time.Sleep(100 * time.Millisecond)
	assert.False(t, sched.IsRunning())
}

// --- Reschedule ---

func TestObsidianSyncScheduler_Reschedule(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = t.TempDir()
	db.settings["obsidian_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(&mockBookDataSource{}, nil, store, nil)
	require.NoError(t, sched.Start(context.Background()))
	assert.True(t, sched.IsRunning())

	// Change schedule and reschedule
	db.settings["obsidian_sync_schedule"] = "*/5 * * * *"
	err := sched.Reschedule()
	require.NoError(t, err)
	assert.True(t, sched.IsRunning())
	sched.Stop()
}

func TestObsidianSyncScheduler_Reschedule_WhenNotRunning(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = t.TempDir()
	db.settings["obsidian_sync_schedule"] = "0 * * * *"
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(&mockBookDataSource{}, nil, store, nil)
	err := sched.Reschedule()
	require.NoError(t, err)
	assert.True(t, sched.IsRunning())
	sched.Stop()
}

// --- RunNow ---

func TestObsidianSyncScheduler_RunNow_Disabled(t *testing.T) {
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "false"
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(&mockBookDataSource{}, nil, store, nil)
	err := sched.RunNow()
	assert.NoError(t, err)
}

func TestObsidianSyncScheduler_RunNow_NoBooks(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")

	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = t.TempDir()
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(&mockBookDataSource{books: nil}, nil, store, nil)
	err := sched.RunNow()
	assert.NoError(t, err)

	// Give runSync goroutine time to complete
	time.Sleep(100 * time.Millisecond)

	// Check status was set
	assert.Contains(t, db.settings["obsidian_sync_last_message"], "No books to export")
}

func TestObsidianSyncScheduler_RunNow_WithBooks(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")

	exportDir := t.TempDir()
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = exportDir
	store := settingsstore.New(db)

	books := []entities.Book{
		{Title: "Test Book", Author: "Author", Highlights: []entities.Highlight{{Text: "highlight"}}},
	}

	sched := NewObsidianSyncScheduler(&mockBookDataSource{books: books}, nil, store, nil)
	err := sched.RunNow()
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, "success", db.settings["obsidian_sync_last_status"])
	assert.Contains(t, db.settings["obsidian_sync_last_message"], "Exported 1 books")
}

func TestObsidianSyncScheduler_RunNow_DBError(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")

	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = t.TempDir()
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(
		&mockBookDataSource{err: fmt.Errorf("db connection failed")},
		nil, store, nil,
	)
	err := sched.RunNow()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "failed", db.settings["obsidian_sync_last_status"])
	assert.Contains(t, db.settings["obsidian_sync_last_message"], "db connection failed")
}

func TestObsidianSyncScheduler_RunNow_NoExportDir(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")

	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	// No export dir set
	store := settingsstore.New(db)

	sched := NewObsidianSyncScheduler(&mockBookDataSource{}, nil, store, nil)
	err := sched.RunNow()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "failed", db.settings["obsidian_sync_last_status"])
	assert.Contains(t, db.settings["obsidian_sync_last_message"], "Export directory not configured")
}

func TestObsidianSyncScheduler_RunNow_WithVocabulary(t *testing.T) {
	t.Setenv("OBSIDIAN_EXPORT_DIR", "")
	t.Setenv("OBSIDIAN_VAULT_DIR", "")

	exportDir := t.TempDir()
	db := newMockSettingsDB()
	db.settings["obsidian_sync_enabled"] = "true"
	db.settings["obsidian_sync_export_dir"] = exportDir
	store := settingsstore.New(db)

	books := []entities.Book{
		{Title: "Book", Author: "Auth", Highlights: []entities.Highlight{{Text: "hl"}}},
	}
	words := []entities.Word{
		{Word: "ephemeral", Status: entities.WordStatusEnriched},
	}

	sched := NewObsidianSyncScheduler(
		&mockBookDataSource{books: books},
		&mockVocabularyReader{words: words, total: 1},
		store, nil,
	)
	err := sched.RunNow()
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, "success", db.settings["obsidian_sync_last_status"])
	assert.Contains(t, db.settings["obsidian_sync_last_message"], "1 vocabulary words")
}
