package database

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/utils"
)

const moonReaderAuthorsMigrationKey = "migration.moonreader_authors_v1"

// fixMoonReaderAuthors re-derives Moon+ book authors from their file paths once, after the filename
// parser learned to drop glued publishers and download-site tags. Without it, the next Moon+ import
// would match on the new title+author and duplicate every affected book.
func fixMoonReaderAuthors(db *gorm.DB) error {
	var done int64
	if err := db.Model(&entities.Setting{}).Where("key = ?", moonReaderAuthorsMigrationKey).Count(&done).Error; err != nil {
		return err
	}
	if done > 0 {
		return nil
	}

	var books []entities.Book
	if err := db.Joins("JOIN sources ON sources.id = books.source_id").
		Where("sources.name = ? AND books.file_path <> ''", "moonreader").
		Find(&books).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, b := range books {
			author := utils.ExtractAuthorFromFilename(b.FilePath, b.Title)
			if author == "" || author == b.Author {
				continue
			}
			var clash int64
			if err := tx.Model(&entities.Book{}).Where("title = ? AND author = ?", b.Title, author).Count(&clash).Error; err != nil {
				return err
			}
			if clash > 0 {
				slog.Warn("Moon+ author fix skipped: a book with that title and author already exists", "id", b.ID, "title", b.Title, "author", author)
				continue
			}
			if err := tx.Model(&entities.Book{}).Where("id = ?", b.ID).Update("author", author).Error; err != nil {
				return fmt.Errorf("update author of book %d: %w", b.ID, err)
			}
			slog.Info("Fixed Moon+ book author", "id", b.ID, "title", b.Title, "old", b.Author, "new", author)
		}
		return tx.Create(&entities.Setting{Key: moonReaderAuthorsMigrationKey, Value: time.Now().UTC().Format(time.RFC3339)}).Error
	})
}
