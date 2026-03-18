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

// Database wraps the GORM connection and provides CRUD for all domain entities.
type Database struct {
	DB *gorm.DB
}

// NewDatabase opens the SQLite database and runs auto-migrations.
func NewDatabase(dbPath string) (*Database, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
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
		&entities.DeletedEntity{},
		&entities.Word{},
		&entities.WordDefinition{},
		&entities.AuditEvent{},
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

// Close releases the underlying database connection.
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

// GetSourceByName retrieves or creates a source by name.
func (d *Database) GetSourceByName(name string) (*entities.Source, error) {
	var source entities.Source
	err := d.DB.Where("name = ?", name).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// GetAllSources returns all registered import sources.
func (d *Database) GetAllSources() ([]entities.Source, error) {
	var sources []entities.Source
	err := d.DB.Find(&sources).Error
	return sources, err
}

// CreateUser inserts a new user record.
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

// GetUserByToken looks up a user by API token hash.
func (d *Database) GetUserByToken(token string) (*entities.User, error) {
	var user entities.User
	err := d.DB.Where("token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID retrieves a user by primary key.
func (d *Database) GetUserByID(id uint) (*entities.User, error) {
	var user entities.User
	err := d.DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername looks up a user by username.
func (d *Database) GetUserByUsername(username string) (*entities.User, error) {
	var user entities.User
	err := d.DB.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// SaveBook upserts a book and its highlights, deduplicating by text + location + timestamp.
// Skips books and highlights that have been permanently deleted.
func (d *Database) SaveBook(book *entities.Book) error {
	// Check if this book was permanently deleted
	deleted, err := d.IsBookDeleted(book.Title, book.Author, book.UserID)
	if err != nil {
		return fmt.Errorf("failed to check if book was deleted: %w", err)
	}
	if deleted {
		log.Printf("Skipping book '%s' by %s: permanently deleted", book.Title, book.Author)
		return nil
	}

	originalSource := d.resolveBookSource(book)
	d.filterHighlights(book)

	// Check if book already exists by title and author for the same user
	var existingBook entities.Book
	result := d.DB.Preload("Highlights").Where("title = ? AND author = ? AND user_id = ?", book.Title, book.Author, book.UserID).First(&existingBook)

	var saveErr error
	switch result.Error {
	case nil:
		// Book exists, merge highlights (deduplicate by text + location)
		book.ID = existingBook.ID

		// Build a map of existing highlights for deduplication
		// key: text+location -> existing highlight (ID + IsFavorite)
		type existingHighlightInfo struct {
			ID         uint
			IsFavorite bool
		}
		existingHighlights := make(map[string]existingHighlightInfo)
		for _, h := range existingBook.Highlights {
			key := fmt.Sprintf("%s|%d|%s", h.Text, h.LocationValue, h.HighlightedAt.Format("2006-01-02 15:04:05"))
			existingHighlights[key] = existingHighlightInfo{ID: h.ID, IsFavorite: h.IsFavorite}
		}

		// Process new highlights: skip duplicates, keep new ones
		var newHighlights []entities.Highlight
		for _, h := range book.Highlights {
			key := fmt.Sprintf("%s|%d|%s", h.Text, h.LocationValue, h.HighlightedAt.Format("2006-01-02 15:04:05"))
			if existing, exists := existingHighlights[key]; exists {
				// Highlight already exists, preserve ID and favourite status
				h.ID = existing.ID
				h.IsFavorite = existing.IsFavorite
			}
			h.BookID = book.ID
			newHighlights = append(newHighlights, h)
		}
		book.Highlights = newHighlights

		// Use Omit to prevent GORM from upserting Source associations
		saveErr = d.DB.Session(&gorm.Session{FullSaveAssociations: true}).Omit("Source", "Highlights.Source").Save(book).Error
	case gorm.ErrRecordNotFound:
		// Book doesn't exist, create it
		// Use Omit to prevent GORM from upserting Source associations
		saveErr = d.DB.Omit("Source", "Highlights.Source").Create(book).Error
	default:
		saveErr = result.Error
	}

	// Restore the source info for callers
	book.Source = originalSource

	return saveErr
}

// resolveBookSource looks up the source by name if SourceID is unset.
func (d *Database) resolveBookSource(book *entities.Book) entities.Source {
	originalSource := book.Source
	if book.SourceID == 0 && book.Source.Name != "" {
		source, err := d.GetSourceByName(book.Source.Name)
		if err == nil && source != nil {
			book.SourceID = source.ID
			originalSource = *source
		}
	}
	return originalSource
}

// filterHighlights resolves source IDs on highlights and removes permanently deleted ones.
func (d *Database) filterHighlights(book *entities.Book) {
	var filtered []entities.Highlight
	for i := range book.Highlights {
		h := &book.Highlights[i]
		if h.SourceID == 0 && h.Source.Name != "" {
			source, err := d.GetSourceByName(h.Source.Name)
			if err == nil && source != nil {
				h.SourceID = source.ID
			}
		}
		deleted, _ := d.IsHighlightDeleted(h.Text, h.LocationValue, h.HighlightedAt, book.UserID)
		if !deleted {
			filtered = append(filtered, *h)
		}
	}
	book.Highlights = filtered
}

// SaveBookForUser saves a book for a specific user.
func (d *Database) SaveBookForUser(book *entities.Book, userID uint) error {
	book.UserID = userID
	return d.SaveBook(book)
}

// GetBookByTitleAndAuthor finds a book by its title and author.
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

// GetBookByTitleAndAuthorForUser finds a book by title, author, and user.
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

// GetBookByID retrieves a book with its highlights by primary key.
func (d *Database) GetBookByID(id uint) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").First(&book, id).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

// GetAllBooks returns all books with their highlights.
func (d *Database) GetAllBooks() ([]entities.Book, error) {
	var books []entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").Find(&books).Error
	return books, err
}

// SearchBooks finds books matching a title/author query string.
func (d *Database) SearchBooks(query string) ([]entities.Book, error) {
	var books []entities.Book
	searchPattern := "%" + query + "%"
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").
		Where("LOWER(title) LIKE LOWER(?) OR LOWER(author) LIKE LOWER(?)", searchPattern, searchPattern).
		Find(&books).Error
	return books, err
}

// GetAllBooksForUser returns all books for a given user.
func (d *Database) GetAllBooksForUser(userID uint) ([]entities.Book, error) {
	var books []entities.Book
	err := d.DB.Preload("Highlights", func(db *gorm.DB) *gorm.DB {
		return db.Order("location_value ASC, highlighted_at ASC")
	}).Preload("Highlights.Tags").Preload("Tags").Preload("Source").Where("user_id = ?", userID).Find(&books).Error
	return books, err
}

// DeleteBook performs a soft delete (sets DeletedAt timestamp).
// Associated highlights are also soft deleted.
func (d *Database) DeleteBook(id uint) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		// Soft delete associated highlights
		if err := tx.Where("book_id = ?", id).Delete(&entities.Highlight{}).Error; err != nil {
			return err
		}
		// Clear book-tag associations
		if err := tx.Exec("DELETE FROM book_tags WHERE book_id = ?", id).Error; err != nil {
			return err
		}
		// Soft delete the book
		return tx.Delete(&entities.Book{}, id).Error
	})
}

// DeleteBookPermanently hard deletes a book, its highlights, and their tag associations.
// Records the deletion to prevent re-import.
func (d *Database) DeleteBookPermanently(id uint, userID uint) error {
	// First get the book to record its key
	var book entities.Book
	if err := d.DB.Unscoped().First(&book, id).Error; err != nil {
		return err
	}

	entityKey := fmt.Sprintf("%s|%s", book.Title, book.Author)

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// Get highlight IDs for tag cleanup
		var highlightIDs []uint
		tx.Model(&entities.Highlight{}).Unscoped().Where("book_id = ?", id).Pluck("id", &highlightIDs)

		// Delete highlight-tag associations
		if len(highlightIDs) > 0 {
			if err := tx.Exec("DELETE FROM highlight_tags WHERE highlight_id IN ?", highlightIDs).Error; err != nil {
				return err
			}
		}

		// Hard delete highlights
		if err := tx.Unscoped().Where("book_id = ?", id).Delete(&entities.Highlight{}).Error; err != nil {
			return err
		}

		// Delete book-tag associations
		if err := tx.Exec("DELETE FROM book_tags WHERE book_id = ?", id).Error; err != nil {
			return err
		}

		// Hard delete the book
		if err := tx.Unscoped().Delete(&entities.Book{}, id).Error; err != nil {
			return err
		}

		// Record the deletion
		deletedEntity := entities.DeletedEntity{
			UserID:     userID,
			EntityType: "book",
			EntityKey:  entityKey,
			SourceID:   book.SourceID,
			DeletedAt:  time.Now(),
		}
		return tx.Create(&deletedEntity).Error
	})
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

// FindBookByISBN looks up a book by ISBN.
func (d *Database) FindBookByISBN(isbn string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Where("isbn = ? AND user_id = ?", isbn, userID).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

// FindBookByFileHash looks up a book by its file hash.
func (d *Database) FindBookByFileHash(hash string, userID uint) (*entities.Book, error) {
	var book entities.Book
	err := d.DB.Where("file_hash = ? AND user_id = ?", hash, userID).First(&book).Error
	if err != nil {
		return nil, err
	}
	return &book, nil
}

// GetHighlightByID retrieves a highlight by primary key.
func (d *Database) GetHighlightByID(id uint) (*entities.Highlight, error) {
	var highlight entities.Highlight
	err := d.DB.Preload("Tags").Preload("Source").First(&highlight, id).Error
	if err != nil {
		return nil, err
	}
	return &highlight, nil
}

// GetHighlightsForBook returns all highlights for a given book.
func (d *Database) GetHighlightsForBook(bookID uint) ([]entities.Highlight, error) {
	var highlights []entities.Highlight
	err := d.DB.Preload("Tags").Where("book_id = ?", bookID).
		Order("location_value ASC, highlighted_at ASC").Find(&highlights).Error
	return highlights, err
}

// GetHighlightsForUser returns paginated highlights for a user.
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

// UpdateHighlight persists changes to a highlight record.
func (d *Database) UpdateHighlight(highlight *entities.Highlight) error {
	return d.DB.Save(highlight).Error
}

// DeleteHighlight performs a soft delete (sets DeletedAt timestamp) and clears tag associations.
func (d *Database) DeleteHighlight(id uint) error {
	return d.DB.Transaction(func(tx *gorm.DB) error {
		// Clear highlight-tag associations
		if err := tx.Exec("DELETE FROM highlight_tags WHERE highlight_id = ?", id).Error; err != nil {
			return err
		}
		// Soft delete the highlight
		return tx.Delete(&entities.Highlight{}, id).Error
	})
}

// DeleteHighlightPermanently hard deletes a highlight and its tag associations.
// Records the deletion to prevent re-import.
func (d *Database) DeleteHighlightPermanently(id uint, userID uint) error {
	// First get the highlight to record its key
	var highlight entities.Highlight
	if err := d.DB.Unscoped().First(&highlight, id).Error; err != nil {
		return err
	}

	entityKey := fmt.Sprintf("%s|%d|%s", highlight.Text, highlight.LocationValue, highlight.HighlightedAt.Format("2006-01-02 15:04:05"))

	return d.DB.Transaction(func(tx *gorm.DB) error {
		// Delete highlight-tag associations
		if err := tx.Exec("DELETE FROM highlight_tags WHERE highlight_id = ?", id).Error; err != nil {
			return err
		}

		// Hard delete the highlight
		if err := tx.Unscoped().Delete(&entities.Highlight{}, id).Error; err != nil {
			return err
		}

		// Record the deletion
		deletedEntity := entities.DeletedEntity{
			UserID:     userID,
			EntityType: "highlight",
			EntityKey:  entityKey,
			SourceID:   highlight.SourceID,
			DeletedAt:  time.Now(),
		}
		return tx.Create(&deletedEntity).Error
	})
}

// IsBookDeleted checks if a book with the given title+author has been permanently deleted.
func (d *Database) IsBookDeleted(title, author string, userID uint) (bool, error) {
	entityKey := fmt.Sprintf("%s|%s", title, author)
	var count int64
	err := d.DB.Model(&entities.DeletedEntity{}).
		Where("entity_type = ? AND entity_key = ? AND (user_id = ? OR user_id = 0)", "book", entityKey, userID).
		Count(&count).Error
	return count > 0, err
}

// IsHighlightDeleted checks if a highlight has been permanently deleted.
func (d *Database) IsHighlightDeleted(text string, locationValue int, highlightedAt time.Time, userID uint) (bool, error) {
	entityKey := fmt.Sprintf("%s|%d|%s", text, locationValue, highlightedAt.Format("2006-01-02 15:04:05"))
	var count int64
	err := d.DB.Model(&entities.DeletedEntity{}).
		Where("entity_type = ? AND entity_key = ? AND (user_id = ? OR user_id = 0)", "highlight", entityKey, userID).
		Count(&count).Error
	return count > 0, err
}

// CreateTag inserts a new tag.
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

// GetOrCreateTag retrieves an existing tag or creates a new one.
func (d *Database) GetOrCreateTag(name string, userID uint) (*entities.Tag, error) {
	var tag entities.Tag
	// Case-insensitive lookup to avoid duplicate tags with different casing
	err := d.DB.Where("LOWER(name) = LOWER(?) AND user_id = ?", name, userID).First(&tag).Error
	if err == gorm.ErrRecordNotFound {
		return d.CreateTag(name, userID)
	}
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// GetTagsForUser returns all tags belonging to a user.
func (d *Database) GetTagsForUser(userID uint) ([]entities.Tag, error) {
	var tags []entities.Tag
	err := d.DB.Where("user_id = ?", userID).Find(&tags).Error
	return tags, err
}

// SearchTags finds tags matching a name query.
func (d *Database) SearchTags(query string, userID uint) ([]entities.Tag, error) {
	var tags []entities.Tag
	searchPattern := "%" + query + "%"
	err := d.DB.Where("user_id = ? AND LOWER(name) LIKE LOWER(?)", userID, searchPattern).Find(&tags).Error
	return tags, err
}

// DeleteTag removes a tag and its associations.
func (d *Database) DeleteTag(id uint) error {
	return d.DB.Delete(&entities.Tag{}, id).Error
}

// IsTagOrphan checks if a tag has no books or highlights associated with it.
func (d *Database) IsTagOrphan(tagID uint) (bool, error) {
	var bookCount int64
	if err := d.DB.Table("book_tags").Where("tag_id = ?", tagID).Count(&bookCount).Error; err != nil {
		return false, err
	}
	if bookCount > 0 {
		return false, nil
	}

	var highlightCount int64
	if err := d.DB.Table("highlight_tags").Where("tag_id = ?", tagID).Count(&highlightCount).Error; err != nil {
		return false, err
	}
	return highlightCount == 0, nil
}

// DeleteTagIfOrphan deletes the tag if it has no associated books or highlights.
func (d *Database) DeleteTagIfOrphan(tagID uint) error {
	orphan, err := d.IsTagOrphan(tagID)
	if err != nil {
		return err
	}
	if orphan {
		return d.DeleteTag(tagID)
	}
	return nil
}

// DeleteOrphanTags removes all tags that have no associated books or highlights.
// Returns the number of tags deleted.
func (d *Database) DeleteOrphanTags() (int64, error) {
	result := d.DB.Exec(`
		DELETE FROM tags
		WHERE id NOT IN (SELECT tag_id FROM book_tags)
		AND id NOT IN (SELECT tag_id FROM highlight_tags)
	`)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// AddTagToHighlight associates a tag with a highlight.
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

// RemoveTagFromHighlight removes a tag-highlight association.
func (d *Database) RemoveTagFromHighlight(highlightID, tagID uint) error {
	var highlight entities.Highlight
	if err := d.DB.First(&highlight, highlightID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := d.DB.First(&tag, tagID).Error; err != nil {
		return err
	}
	if err := d.DB.Model(&highlight).Association("Tags").Delete(&tag); err != nil {
		return err
	}
	return d.DeleteTagIfOrphan(tagID)
}

// AddTagToBook associates a tag with a book.
func (d *Database) AddTagToBook(bookID, tagID uint) error {
	var book entities.Book
	if err := d.DB.First(&book, bookID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := d.DB.First(&tag, tagID).Error; err != nil {
		return err
	}
	return d.DB.Model(&book).Association("Tags").Append(&tag)
}

// RemoveTagFromBook removes a tag-book association.
func (d *Database) RemoveTagFromBook(bookID, tagID uint) error {
	var book entities.Book
	if err := d.DB.First(&book, bookID).Error; err != nil {
		return err
	}
	var tag entities.Tag
	if err := d.DB.First(&tag, tagID).Error; err != nil {
		return err
	}
	if err := d.DB.Model(&book).Association("Tags").Delete(&tag); err != nil {
		return err
	}
	return d.DeleteTagIfOrphan(tagID)
}

// GetBooksByTag returns all books with a given tag.
func (d *Database) GetBooksByTag(tagID uint, userID uint) ([]entities.Book, error) {
	var tag entities.Tag
	if err := d.DB.First(&tag, tagID).Error; err != nil {
		return nil, err
	}

	var books []entities.Book

	// Find books that either have the tag directly OR have highlights with the tag
	subQuery := `
		books.id IN (
			SELECT book_id FROM book_tags WHERE tag_id = ?
		) OR books.id IN (
			SELECT book_id FROM highlights
			JOIN highlight_tags ON highlights.id = highlight_tags.highlight_id
			WHERE highlight_tags.tag_id = ?
		)
	`

	query := d.DB.
		Preload("Highlights", func(db *gorm.DB) *gorm.DB {
			return db.Order("location_value ASC, highlighted_at ASC")
		}).
		Preload("Source").
		Preload("Tags").
		Where(subQuery, tagID, tagID)

	// Only filter by user_id if specified (non-zero)
	if userID > 0 {
		query = query.Where("books.user_id = ?", userID)
	}

	err := query.Find(&books).Error
	return books, err
}

// GetTagByID retrieves a tag by primary key.
func (d *Database) GetTagByID(id uint) (*entities.Tag, error) {
	var tag entities.Tag
	err := d.DB.First(&tag, id).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// CreateImportSession records a new import operation.
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

// UpdateImportSession persists changes to an import session.
func (d *Database) UpdateImportSession(session *entities.ImportSession) error {
	return d.DB.Save(session).Error
}

// GetImportSession retrieves an import session by ID.
func (d *Database) GetImportSession(id uint) (*entities.ImportSession, error) {
	var session entities.ImportSession
	err := d.DB.Preload("Source").First(&session, id).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetImportSessionsForUser returns recent import sessions for a user.
func (d *Database) GetImportSessionsForUser(userID uint) ([]entities.ImportSession, error) {
	var sessions []entities.ImportSession
	err := d.DB.Preload("Source").Where("user_id = ?", userID).Order("started_at DESC").Find(&sessions).Error
	return sessions, err
}

// GetStatsForUser returns book/highlight counts for a user.
func (d *Database) GetStatsForUser(userID uint) (totalBooks int64, totalHighlights int64, err error) {
	err = d.DB.Model(&entities.Book{}).Where("user_id = ?", userID).Count(&totalBooks).Error
	if err != nil {
		return
	}
	err = d.DB.Model(&entities.Highlight{}).Where("user_id = ?", userID).Count(&totalHighlights).Error
	return
}

// GetStats returns aggregate book/highlight counts.
func (d *Database) GetStats() (totalBooks int64, totalHighlights int64, err error) {
	err = d.DB.Model(&entities.Book{}).Count(&totalBooks).Error
	if err != nil {
		return
	}
	err = d.DB.Model(&entities.Highlight{}).Count(&totalHighlights).Error
	return
}

// GetSetting retrieves a setting value by key.
func (d *Database) GetSetting(key string) (*entities.Setting, error) {
	var setting entities.Setting
	err := d.DB.Where("key = ?", key).First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// SetSetting creates or updates a setting.
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

// DeleteSetting removes a setting by key.
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
