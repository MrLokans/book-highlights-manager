package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/testutil"
)

func TestRepository_StartSync(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

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
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.StartSync(50)
	require.NoError(t, err)

	err = repo.UpdateProgress(25, 20, 5, 0, "Book A")
	require.NoError(t, err)

	err = repo.StartSync(100)
	require.NoError(t, err)

	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, 100, progress.TotalItems)
	assert.Equal(t, 0, progress.Processed)
	assert.Equal(t, "", progress.CurrentItem)
}

func TestRepository_UpdateProgress(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

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
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

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
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

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
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.False(t, running)
}

func TestRepository_IsSyncRunning_Running(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.StartSync(10)
	require.NoError(t, err)

	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.True(t, running)
}

func TestRepository_IsSyncRunning_Completed(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.StartSync(10)
	require.NoError(t, err)
	err = repo.CompleteSync(true, "")
	require.NoError(t, err)

	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.False(t, running)
}

func TestRepository_IsSyncRunning_StaleSync(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.StartSync(10)
	require.NoError(t, err)

	// Manually set updated_at to 15 minutes ago to simulate stale sync
	repo.db.Model(&entities.SyncProgress{}).
		Where("sync_type = ?", entities.SyncTypeMetadata).
		Update("updated_at", time.Now().Add(-15*time.Minute))

	running, err := repo.IsSyncRunning()
	require.NoError(t, err)
	assert.False(t, running)

	// Verify it was marked as failed
	progress, err := repo.GetSyncProgress()
	require.NoError(t, err)
	assert.Equal(t, entities.SyncStatusFailed, progress.Status)
}
