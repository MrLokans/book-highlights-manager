// Package sync provides database operations for sync progress tracking.
//
// This package implements the ProgressReporter interface used by the metadata enricher.
//
// # Interface Implementation
//
//	var _ metadata.ProgressReporter = (*Repository)(nil)
//
// # Usage
//
//	repo := sync.NewRepository(db)
//	progress, err := repo.StartSync(entities.SyncTypeMetadata, 100)
package sync

import (
	"time"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// Repository handles all sync progress database operations.
type Repository struct {
	db       *gorm.DB
	syncType entities.SyncType
}

// NewRepository creates a new sync repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, syncType: entities.SyncTypeMetadata}
}

// NewRepositoryWithType creates a sync repository for a specific sync type.
func NewRepositoryWithType(db *gorm.DB, syncType entities.SyncType) *Repository {
	return &Repository{db: db, syncType: syncType}
}

// GetSyncProgress retrieves the sync progress for the configured sync type.
func (r *Repository) GetSyncProgress() (*entities.SyncProgress, error) {
	var progress entities.SyncProgress
	err := r.db.Where("sync_type = ?", r.syncType).First(&progress).Error
	if err != nil {
		return nil, err
	}
	return &progress, nil
}

// StartSync creates or resets a sync progress record.
// Implements ProgressReporter.StartSync.
func (r *Repository) StartSync(totalItems int) error {
	var progress entities.SyncProgress
	result := r.db.Where("sync_type = ?", r.syncType).First(&progress)

	now := time.Now()
	if result.Error == gorm.ErrRecordNotFound {
		progress = entities.SyncProgress{
			SyncType:   r.syncType,
			Status:     entities.SyncStatusRunning,
			TotalItems: totalItems,
			StartedAt:  now,
			UpdatedAt:  now,
		}
		return r.db.Create(&progress).Error
	} else if result.Error != nil {
		return result.Error
	}

	// Reset existing record
	progress.Status = entities.SyncStatusRunning
	progress.TotalItems = totalItems
	progress.Processed = 0
	progress.Succeeded = 0
	progress.Failed = 0
	progress.Skipped = 0
	progress.CurrentItem = ""
	progress.Error = ""
	progress.StartedAt = now
	progress.UpdatedAt = now
	progress.CompletedAt = nil

	return r.db.Save(&progress).Error
}

// UpdateProgress updates the progress of an ongoing sync.
// Implements ProgressReporter.UpdateProgress.
func (r *Repository) UpdateProgress(processed, succeeded, failed, skipped int, currentItem string) error {
	return r.db.Model(&entities.SyncProgress{}).
		Where("sync_type = ?", r.syncType).
		Updates(map[string]any{
			"processed":    processed,
			"succeeded":    succeeded,
			"failed":       failed,
			"skipped":      skipped,
			"current_item": currentItem,
			"updated_at":   time.Now(),
		}).Error
}

// CompleteSync marks a sync as completed or failed.
// Implements ProgressReporter.CompleteSync.
func (r *Repository) CompleteSync(succeeded bool, errorMsg string) error {
	now := time.Now()
	status := entities.SyncStatusCompleted
	if !succeeded {
		status = entities.SyncStatusFailed
	}

	updates := map[string]any{
		"status":       status,
		"current_item": "",
		"updated_at":   now,
		"completed_at": now,
	}
	if errorMsg != "" {
		updates["error"] = errorMsg
	}
	return r.db.Model(&entities.SyncProgress{}).
		Where("sync_type = ?", r.syncType).
		Updates(updates).Error
}

// IsSyncRunning checks if a sync is currently in progress.
// A sync is considered stale if not updated in 10 minutes.
// Implements ProgressReporter.IsSyncRunning.
func (r *Repository) IsSyncRunning() (bool, error) {
	var progress entities.SyncProgress
	err := r.db.Where("sync_type = ? AND status = ?", r.syncType, entities.SyncStatusRunning).First(&progress).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Consider sync stale if not updated in 10 minutes
	staleThreshold := time.Now().Add(-10 * time.Minute)
	if progress.UpdatedAt.Before(staleThreshold) {
		_ = r.CompleteSync(false, "sync was interrupted")
		return false, nil
	}

	return true, nil
}
