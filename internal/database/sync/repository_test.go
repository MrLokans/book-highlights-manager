package sync

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestDB(t *testing.T) (*Repository, func()) {
	dbPath := "./test_sync_" + t.Name() + ".db"

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&entities.SyncProgress{})
	require.NoError(t, err)

	repo := NewRepository(db)

	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		os.Remove(dbPath)
	}

	return repo, cleanup
}

func TestRepository_StartSync(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.StartSync(100)
	require.NoError(t, err)

	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, entities.SyncTypeMetadata, progress.SyncType)
	assert.Equal(t, entities.SyncStatusRunning, progress.Status)
	assert.Equal(t, 100, progress.TotalItems)
	assert.Equal(t, 0, progress.Processed)
}

func TestRepository_StartSync_Reset(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Start first sync
	err := repo.StartSync(50)
	require.NoError(t, err)

	// Update progress
	err = repo.UpdateProgress(25, 20, 5, 0, "Book A")
	require.NoError(t, err)

	// Start new sync should reset
	err = repo.StartSync(100)
	require.NoError(t, err)

	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, 100, progress.TotalItems)
	assert.Equal(t, 0, progress.Processed)
	assert.Equal(t, "", progress.CurrentItem)
}

func TestRepository_UpdateProgress(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.StartSync(100)
	require.NoError(t, err)

	err = repo.UpdateProgress(50, 45, 3, 2, "Current Book")
	require.NoError(t, err)

	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, 50, progress.Processed)
	assert.Equal(t, 45, progress.Succeeded)
	assert.Equal(t, 3, progress.Failed)
	assert.Equal(t, 2, progress.Skipped)
	assert.Equal(t, "Current Book", progress.CurrentItem)
}

func TestRepository_CompleteSync_Success(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.StartSync(10)
	require.NoError(t, err)

	err = repo.CompleteSync(true, "")
	require.NoError(t, err)

	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, entities.SyncStatusCompleted, progress.Status)
	assert.NotNil(t, progress.CompletedAt)
}

func TestRepository_CompleteSync_Failure(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.StartSync(10)
	require.NoError(t, err)

	err = repo.CompleteSync(false, "some error occurred")
	require.NoError(t, err)

	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, entities.SyncStatusFailed, progress.Status)
	assert.Equal(t, "some error occurred", progress.Error)
}

func TestRepository_IsSyncRunning_NotRunning(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.False(t, running)
}

func TestRepository_IsSyncRunning_Running(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.StartSync(10)
	require.NoError(t, err)

	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.True(t, running)
}

func TestRepository_IsSyncRunning_Completed(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.StartSync(10)
	require.NoError(t, err)
	err = repo.CompleteSync(true, "")
	require.NoError(t, err)

	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.False(t, running)
}

func TestRepository_IsSyncRunning_StaleSync(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Start sync
	err := repo.StartSync(10)
	require.NoError(t, err)

	// Manually set updated_at to 15 minutes ago to simulate stale sync
	repo.db.Model(&entities.SyncProgress{}).
		Where("sync_type = ?", entities.SyncTypeMetadata).
		Update("updated_at", time.Now().Add(-15*time.Minute))

	// Should detect as not running (stale) and mark as failed
	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.False(t, running)

	// Verify it was marked as failed
	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, entities.SyncStatusFailed, progress.Status)
}
