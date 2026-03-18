package database

import (
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/metadata"
)

// BooksMetadataReader provides book operations needed by metadata enrichment.
type BooksMetadataReader interface {
	GetBookByID(id uint) (*entities.Book, error)
	UpdateBookMetadata(id uint, fields map[string]any) error
	GetBooksMissingMetadata() ([]entities.Book, error)
}

// MetadataUpdater wraps BooksMetadataReader to implement metadata.BookUpdater.
type MetadataUpdater struct {
	books BooksMetadataReader
}

// NewMetadataUpdater creates a MetadataUpdater wrapping the given books reader.
func NewMetadataUpdater(books BooksMetadataReader) *MetadataUpdater {
	return &MetadataUpdater{books: books}
}

// GetBookByID delegates to the underlying books reader.
func (m *MetadataUpdater) GetBookByID(id uint) (*entities.Book, error) {
	return m.books.GetBookByID(id)
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

	return m.books.UpdateBookMetadata(id, updates)
}

// GetBooksMissingMetadata returns books missing cover, publisher, or year.
func (m *MetadataUpdater) GetBooksMissingMetadata() ([]entities.Book, error) {
	return m.books.GetBooksMissingMetadata()
}

// SyncProgressDB provides sync progress operations needed by MetadataSyncProgress.
type SyncProgressDB interface {
	StartSync(totalItems int) error
	UpdateProgress(processed, succeeded, failed, skipped int, currentItem string) error
	CompleteSync(succeeded bool, errorMsg string) error
	IsSyncRunning() (bool, error)
	GetSyncProgress() (*entities.SyncProgress, error)
}

// MetadataSyncProgress implements metadata.ProgressReporter for tracking sync progress.
type MetadataSyncProgress struct {
	sync SyncProgressDB
}

// NewMetadataSyncProgress creates a new MetadataSyncProgress.
func NewMetadataSyncProgress(sync SyncProgressDB) *MetadataSyncProgress {
	return &MetadataSyncProgress{sync: sync}
}

// StartSync begins tracking a new sync operation.
func (p *MetadataSyncProgress) StartSync(totalItems int) error {
	return p.sync.StartSync(totalItems)
}

// UpdateProgress updates the current sync progress.
func (p *MetadataSyncProgress) UpdateProgress(processed, succeeded, failed, skipped int, currentItem string) error {
	return p.sync.UpdateProgress(processed, succeeded, failed, skipped, currentItem)
}

// CompleteSync marks the sync as completed.
func (p *MetadataSyncProgress) CompleteSync(succeeded bool, errorMsg string) error {
	return p.sync.CompleteSync(succeeded, errorMsg)
}

// IsSyncRunning checks if a metadata sync is currently running.
func (p *MetadataSyncProgress) IsSyncRunning() (bool, error) {
	return p.sync.IsSyncRunning()
}

// GetProgress returns the current sync progress.
func (p *MetadataSyncProgress) GetProgress() (*entities.SyncProgress, error) {
	return p.sync.GetSyncProgress()
}
