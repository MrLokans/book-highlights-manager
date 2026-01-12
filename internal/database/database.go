package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mrlokans/assistant/internal/entities"
)

var defaultSources = []entities.Source{
	{Name: "readwise", DisplayName: "Readwise"},
	{Name: "kindle", DisplayName: "Amazon Kindle"},
	{Name: "apple_books", DisplayName: "Apple Books"},
	{Name: "kobo", DisplayName: "Kobo"},
	{Name: "moonreader", DisplayName: "Moon+ Reader"},
	{Name: "libby", DisplayName: "Libby/OverDrive"},
	{Name: "google_play", DisplayName: "Google Play Books"},
	{Name: "calibre", DisplayName: "Calibre"},
	{Name: "instapaper", DisplayName: "Instapaper"},
	{Name: "pocket", DisplayName: "Pocket"},
	{Name: "manual", DisplayName: "Manual Import"},
}

type Database struct {
	DB *gorm.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate all entities
	err = db.AutoMigrate(
		&entities.Source{},
		&entities.User{},
		&entities.Book{},
		&entities.Highlight{},
		&entities.Tag{},
		&entities.ImportSession{},
		&entities.Setting{},
		&entities.SyncProgress{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	database := &Database{DB: db}

	// Seed default sources
	if err := database.seedSources(); err != nil {
		return nil, fmt.Errorf("failed to seed sources: %w", err)
	}

	log.Printf("Database initialized successfully at %s", dbPath)

	return database, nil
}

func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *Database) seedSources() error {
	for _, source := range defaultSources {
		var existing entities.Source
		result := d.DB.Where("name = ?", source.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			if err := d.DB.Create(&source).Error; err != nil {
				return fmt.Errorf("failed to create source %s: %w", source.Name, err)
			}
			log.Printf("Created source: %s", source.DisplayName)
		}
	}
	return nil
}

func (d *Database) GetSourceByName(name string) (*entities.Source, error) {
	var source entities.Source
	err := d.DB.Where("name = ?", name).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (d *Database) GetAllSources() ([]entities.Source, error) {
	var sources []entities.Source
	err := d.DB.Find(&sources).Error
	return sources, err
}

func (d *Database) CreateUser(username, email string) (*entities.User, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	user := &entities.User{
		Username: username,
		Email:    email,
		Token:    token,
	}

	if err := d.DB.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (d *Database) GetUserByToken(token string) (*entities.User, error) {
	var user entities.User
	err := d.DB.Where("token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *Database) GetUserByID(id uint) (*entities.User, error) {
	var user entities.User
	err := d.DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *Database) GetUserByUsername(username string) (*entities.User, error) {
	var user entities.User
	err := d.DB.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Upserts a book and its highlights, deduplicating by text + location + timestamp
func (d *Database) SaveBook(book *entities.Book) error {
	// If Source.Name is set but SourceID is 0, look up the source
	// Preserve the original source info for callers who need it after save
	originalSource := book.Source
	if book.SourceID == 0 && book.Source.Name != "" {
		source, err := d.GetSourceByName(book.Source.Name)
		if err == nil && source != nil {
			book.SourceID = source.ID
			originalSource = *source
		}
	}

	// Also fix SourceID for all highlights
	for i := range book.Highlights {
		if book.Highlights[i].SourceID == 0 && book.Highlights[i].Source.Name != "" {
			source, err := d.GetSourceByName(book.Highlights[i].Source.Name)
			if err == nil && source != nil {
				book.Highlights[i].SourceID = source.ID
			}
		}
	}

	// Check if book already exists by title and author for the same user
	var existingBook entities.Book
	result := d.DB.Preload("Highlights").Where("title = ? AND author = ? AND user_id = ?", book.Title, book.Author, book.UserID).First(&existingBook)

	var saveErr error
	if result.Error == nil {
		// Book exists, merge highlights (deduplicate by text + location)
		book.ID = existingBook.ID

		// Build a map of existing highlights for deduplication
		existingHighlights := make(map[string]uint) // key: text+location -> highlight ID
		for _, h := range existingBook.Highlights {
			key := fmt.Sprintf("%s|%d|%s", h.Text, h.LocationValue, h.HighlightedAt.Format("2006-01-02 15:04:05"))
			existingHighlights[key] = h.ID
		}

		// Process new highlights: skip duplicates, keep new ones
		var newHighlights []entities.Highlight
		for _, h := range book.Highlights {
			key := fmt.Sprintf("%s|%d|%s", h.Text, h.LocationValue, h.HighlightedAt.Format("2006-01-02 15:04:05"))
			if existingID, exists := existingHighlights[key]; exists {
				// Highlight already exists, update the ID to reference existing one
				h.ID = existingID
			}
			h.BookID = book.ID
			newHighlights = append(newHighlights, h)
		}
		book.Highlights = newHighlights

		// Use Omit to prevent GORM from upserting Source associations
		saveErr = d.DB.Session(&gorm.Session{FullSaveAssociations: true}).Omit("Source", "Highlights.Source").Save(book).Error
	} else if result.Error == gorm.ErrRecordNotFound {
		// Book doesn't exist, create it
		// Use Omit to prevent GORM from upserting Source associations
		saveErr = d.DB.Omit("Source", "Highlights.Source").Create(book).Error
	} else {
		saveErr = result.Error
	}

	// Restore the source info for callers
	book.Source = originalSource

	return saveErr
}

func (d *Database) SaveBookForUser(book *entities.Book, userID uint) error {
	book.UserID = userID
	return d.SaveBook(book)
}

func (d *Database) GetBookByTitleAndAuthor(title, author string) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Source").Where("title = ? AND author = ?", title, author).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (d *Database) GetBookByTitleAndAuthorForUser(title, author string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Source").
		Where("title = ? AND author = ? AND user_id = ?", title, author, userID).
		First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (d *Database) GetBookByID(id uint) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Source").First(&book, id).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (d *Database) GetAllBooks() ([]entities.Book, error) {
	var books []entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Source").Find(&books).Error
	return books, err
}

func (d *Database) SearchBooks(query string) ([]entities.Book, error) {
	var books []entities.Book
	searchPattern := "%" + query + "%"
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Source").
		Where("LOWER(title) LIKE LOWER(?) OR LOWER(author) LIKE LOWER(?)", searchPattern, searchPattern).
		Find(&books).Error
	return books, err
}

func (d *Database) GetAllBooksForUser(userID uint) ([]entities.Book, error) {
	var books []entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Source").Where("user_id = ?", userID).Find(&books).Error
	return books, err
}

func (d *Database) DeleteBook(id uint) error {
	return d.DB.Delete(&entities.Book{}, id).Error
}

// UpdateBookMetadata updates specific metadata fields on a book without affecting other data.
func (d *Database) UpdateBookMetadata(id uint, fields map[string]any) error {
	return d.DB.Model(&entities.Book{}).Where("id = ?", id).Updates(fields).Error
}

// GetBooksMissingMetadata returns books that have no cover URL, publisher, or publication year.
func (d *Database) GetBooksMissingMetadata() ([]entities.Book, error) {
	var books []entities.Book
	err := d.DB.Where(
		"cover_url = '' OR cover_url IS NULL OR publisher = '' OR publisher IS NULL OR publication_year = 0 OR publication_year IS NULL",
	).Find(&books).Error
	return books, err
}

func (d *Database) FindBookByISBN(isbn string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Where("isbn = ? AND user_id = ?", isbn, userID).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (d *Database) FindBookByFileHash(hash string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Where("file_hash = ? AND user_id = ?", hash, userID).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

func (d *Database) GetHighlightByID(id uint) (*entities.Highlight, error) {
	var highlight entities.Highlight
	err := d.DB.Preload("Tags").Preload("Source").First(&highlight, id).Error
	if err != nil {
		return nil, err
	}
	return &highlight, nil
}

func (d *Database) GetHighlightsForBook(bookID uint) ([]entities.Highlight, error) {
	var highlights []entities.Highlight
	err := d.DB.Preload("Tags").Where("book_id = ?", bookID).
		Order("location_value ASC, highlighted_at ASC").Find(&highlights).Error
	return highlights, err
}

func (d *Database) GetHighlightsForUser(userID uint, limit, offset int) ([]entities.Highlight, error) {
	var highlights []entities.Highlight
	query := d.DB.Preload("Tags").Preload("Source").Where("user_id = ?", userID).Order("highlighted_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&highlights).Error
	return highlights, err
}

func (d *Database) UpdateHighlight(highlight *entities.Highlight) error {
	return d.DB.Save(highlight).Error
}

func (d *Database) DeleteHighlight(id uint) error {
	return d.DB.Delete(&entities.Highlight{}, id).Error
}

func (d *Database) CreateTag(name string, userID uint) (*entities.Tag, error) {
	tag := &entities.Tag{
		Name:   name,
		UserID: userID,
	}
	if err := d.DB.Create(tag).Error; err != nil {
		return nil, err
	}
	return tag, nil
}

func (d *Database) GetOrCreateTag(name string, userID uint) (*entities.Tag, error) {
	var tag entities.Tag
	err := d.DB.Where("name = ? AND user_id = ?", name, userID).First(&tag).Error
	if err == gorm.ErrRecordNotFound {
		return d.CreateTag(name, userID)
	}
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

func (d *Database) GetTagsForUser(userID uint) ([]entities.Tag, error) {
	var tags []entities.Tag
	err := d.DB.Where("user_id = ?", userID).Find(&tags).Error
	return tags, err
}

func (d *Database) DeleteTag(id uint) error {
	return d.DB.Delete(&entities.Tag{}, id).Error
}

func (d *Database) AddTagToHighlight(highlightID, tagID uint) error {
	var highlight entities.Highlight
	if err := d.DB.First(&highlight, highlightID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := d.DB.First(&tag, tagID).Error; err != nil {
		return err
	}
	return d.DB.Model(&highlight).Association("Tags").Append(&tag)
}

func (d *Database) RemoveTagFromHighlight(highlightID, tagID uint) error {
	var highlight entities.Highlight
	if err := d.DB.First(&highlight, highlightID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := d.DB.First(&tag, tagID).Error; err != nil {
		return err
	}
	return d.DB.Model(&highlight).Association("Tags").Delete(&tag)
}

func (d *Database) CreateImportSession(userID, sourceID uint) (*entities.ImportSession, error) {
	session := &entities.ImportSession{
		UserID:   userID,
		SourceID: sourceID,
		Status:   entities.ImportStatusPending,
	}
	if err := d.DB.Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

func (d *Database) UpdateImportSession(session *entities.ImportSession) error {
	return d.DB.Save(session).Error
}

func (d *Database) GetImportSession(id uint) (*entities.ImportSession, error) {
	var session entities.ImportSession
	err := d.DB.Preload("Source").First(&session, id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (d *Database) GetImportSessionsForUser(userID uint) ([]entities.ImportSession, error) {
	var sessions []entities.ImportSession
	err := d.DB.Preload("Source").Where("user_id = ?", userID).Order("started_at DESC").Find(&sessions).Error
	return sessions, err
}

func (d *Database) GetStatsForUser(userID uint) (totalBooks int64, totalHighlights int64, err error) {
	err = d.DB.Model(&entities.Book{}).Where("user_id = ?", userID).Count(&totalBooks).Error
	if err != nil {
		return
	}
	err = d.DB.Model(&entities.Highlight{}).Where("user_id = ?", userID).Count(&totalHighlights).Error
	return
}

func (d *Database) GetStats() (totalBooks int64, totalHighlights int64, err error) {
	err = d.DB.Model(&entities.Book{}).Count(&totalBooks).Error
	if err != nil {
		return
	}
	err = d.DB.Model(&entities.Highlight{}).Count(&totalHighlights).Error
	return
}

func (d *Database) GetSetting(key string) (*entities.Setting, error) {
	var setting entities.Setting
	err := d.DB.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

func (d *Database) SetSetting(key, value string) error {
	var setting entities.Setting
	result := d.DB.Where("key = ?", key).First(&setting)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new setting
		setting = entities.Setting{
			Key:   key,
			Value: value,
		}
		return d.DB.Create(&setting).Error
	} else if result.Error != nil {
		return result.Error
	}

	// Update existing setting
	setting.Value = value
	return d.DB.Save(&setting).Error
}

func (d *Database) DeleteSetting(key string) error {
	return d.DB.Where("key = ?", key).Delete(&entities.Setting{}).Error
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GetSyncProgress retrieves the sync progress for a given sync type.
func (d *Database) GetSyncProgress(syncType entities.SyncType) (*entities.SyncProgress, error) {
	var progress entities.SyncProgress
	err := d.DB.Where("sync_type = ?", syncType).First(&progress).Error
	if err != nil {
		return nil, err
	}
	return &progress, nil
}

// StartSyncProgress creates or resets a sync progress record for the given type.
func (d *Database) StartSyncProgress(syncType entities.SyncType, totalItems int) (*entities.SyncProgress, error) {
	var progress entities.SyncProgress
	result := d.DB.Where("sync_type = ?", syncType).First(&progress)

	now := time.Now()
	if result.Error == gorm.ErrRecordNotFound {
		progress = entities.SyncProgress{
			SyncType:   syncType,
			Status:     entities.SyncStatusRunning,
			TotalItems: totalItems,
			StartedAt:  now,
			UpdatedAt:  now,
		}
		if err := d.DB.Create(&progress).Error; err != nil {
			return nil, err
		}
		return &progress, nil
	} else if result.Error != nil {
		return nil, result.Error
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

	if err := d.DB.Save(&progress).Error; err != nil {
		return nil, err
	}
	return &progress, nil
}

// UpdateSyncProgress updates the progress of an ongoing sync.
func (d *Database) UpdateSyncProgress(syncType entities.SyncType, processed, succeeded, failed, skipped int, currentItem string) error {
	return d.DB.Model(&entities.SyncProgress{}).
		Where("sync_type = ?", syncType).
		Updates(map[string]any{
			"processed":    processed,
			"succeeded":    succeeded,
			"failed":       failed,
			"skipped":      skipped,
			"current_item": currentItem,
			"updated_at":   time.Now(),
		}).Error
}

// CompleteSyncProgress marks a sync as completed or failed.
func (d *Database) CompleteSyncProgress(syncType entities.SyncType, status entities.SyncStatus, errorMsg string) error {
	now := time.Now()
	updates := map[string]any{
		"status":       status,
		"current_item": "",
		"updated_at":   now,
		"completed_at": now,
	}
	if errorMsg != "" {
		updates["error"] = errorMsg
	}
	return d.DB.Model(&entities.SyncProgress{}).
		Where("sync_type = ?", syncType).
		Updates(updates).Error
}

// IsMetadataSyncRunning checks if a metadata sync is currently in progress.
// A sync is considered stale if it hasn't been updated in more than 10 minutes.
func (d *Database) IsMetadataSyncRunning() (bool, error) {
	var progress entities.SyncProgress
	err := d.DB.Where("sync_type = ? AND status = ?", entities.SyncTypeMetadata, entities.SyncStatusRunning).First(&progress).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	// Consider sync stale if not updated in 10 minutes (cleanup interrupted syncs)
	staleThreshold := time.Now().Add(-10 * time.Minute)
	if progress.UpdatedAt.Before(staleThreshold) {
		// Mark the stale sync as failed
		_ = d.CompleteSyncProgress(entities.SyncTypeMetadata, entities.SyncStatusFailed, "sync was interrupted")
		return false, nil
	}

	return true, nil
}
