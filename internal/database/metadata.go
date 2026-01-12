package database

import (
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/metadata"
)

// MetadataUpdater wraps the Database to implement metadata.BookUpdater interface.
type MetadataUpdater struct {
	db *Database
}

// NewMetadataUpdater creates a MetadataUpdater wrapping the given database.
func NewMetadataUpdater(db *Database) *MetadataUpdater {
	return &MetadataUpdater{db: db}
}

// GetBookByID delegates to the underlying database.
func (m *MetadataUpdater) GetBookByID(id uint) (*entities.Book, error) {
	return m.db.GetBookByID(id)
}

// UpdateBookMetadata converts BookUpdateFields to a map and updates the book.
func (m *MetadataUpdater) UpdateBookMetadata(id uint, fields metadata.BookUpdateFields) error {
	updates := make(map[string]any)

	if fields.ISBN != nil {
		updates["isbn"] = *fields.ISBN
	}
	if fields.CoverURL != nil {
		updates["cover_url"] = *fields.CoverURL
	}
	if fields.Publisher != nil {
		updates["publisher"] = *fields.Publisher
	}
	if fields.PublicationYear != nil {
		updates["publication_year"] = *fields.PublicationYear
	}

	if len(updates) == 0 {
		return nil
	}

	return m.db.UpdateBookMetadata(id, updates)
}

// GetBooksMissingMetadata returns books missing cover, publisher, or year.
func (m *MetadataUpdater) GetBooksMissingMetadata() ([]entities.Book, error) {
	return m.db.GetBooksMissingMetadata()
}

// MetadataSyncProgress implements metadata.ProgressReporter for tracking sync progress.
type MetadataSyncProgress struct {
	db *Database
}

// NewMetadataSyncProgress creates a new MetadataSyncProgress.
func NewMetadataSyncProgress(db *Database) *MetadataSyncProgress {
	return &MetadataSyncProgress{db: db}
}

// StartSync begins tracking a new sync operation.
func (p *MetadataSyncProgress) StartSync(totalItems int) error {
	_, err := p.db.StartSyncProgress(entities.SyncTypeMetadata, totalItems)
	return err
}

// UpdateProgress updates the current sync progress.
func (p *MetadataSyncProgress) UpdateProgress(processed, succeeded, failed, skipped int, currentItem string) error {
	return p.db.UpdateSyncProgress(entities.SyncTypeMetadata, processed, succeeded, failed, skipped, currentItem)
}

// CompleteSync marks the sync as completed.
func (p *MetadataSyncProgress) CompleteSync(succeeded bool, errorMsg string) error {
	status := entities.SyncStatusCompleted
	if !succeeded {
		status = entities.SyncStatusFailed
	}
	return p.db.CompleteSyncProgress(entities.SyncTypeMetadata, status, errorMsg)
}

// IsSyncRunning checks if a metadata sync is currently running.
func (p *MetadataSyncProgress) IsSyncRunning() (bool, error) {
	return p.db.IsMetadataSyncRunning()
}

// GetProgress returns the current sync progress.
func (p *MetadataSyncProgress) GetProgress() (*entities.SyncProgress, error) {
	return p.db.GetSyncProgress(entities.SyncTypeMetadata)
}
