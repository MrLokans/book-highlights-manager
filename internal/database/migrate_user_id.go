package database

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// legacyUserTables lists the tables that carried a user_id column (and users foreign key) in multi-user mode.
var legacyUserTables = []struct {
	model      any
	table      string
	foreignKey string
}{
	{&entities.Book{}, "books", "fk_books_user"},
	{&entities.Highlight{}, "highlights", "fk_highlights_user"},
	{&entities.Tag{}, "tags", "fk_tags_user"},
	{&entities.ImportSession{}, "import_sessions", "fk_import_sessions_user"},
	{&entities.Word{}, "words", "fk_words_user"},
	{&entities.DeletedEntity{}, "deleted_entities", ""},
}

func needsUserIDMigration(db *gorm.DB) bool {
	m := db.Migrator()
	for _, t := range legacyUserTables {
		if m.HasTable(t.table) && m.HasColumn(t.table, "user_id") {
			return true
		}
	}
	return false
}

// dropLegacyUserColumns removes the user_id columns left over from multi-user mode.
// It must run before AutoMigrate: rebuilding a table drops its indexes, which AutoMigrate then recreates,
// and the new unique index on tags.name needs duplicate tag names merged first.
func dropLegacyUserColumns(db *gorm.DB, dbPath string) error {
	if !needsUserIDMigration(db) {
		return nil
	}

	backupPath := dbPath + ".pre-user-id-migration.bak"
	if err := db.Exec("VACUUM INTO ?", backupPath).Error; err != nil {
		return fmt.Errorf("failed to back up database before migration: %w", err)
	}
	slog.Info("Removing legacy user_id columns", "backup", backupPath)

	return db.Transaction(func(tx *gorm.DB) error {
		m := tx.Migrator()
		if m.HasTable("tags") && m.HasColumn("tags", "user_id") {
			if err := mergeDuplicateTags(tx); err != nil {
				return fmt.Errorf("failed to merge duplicate tags: %w", err)
			}
		}

		for _, t := range legacyUserTables {
			if m.HasTable(t.table) && m.HasColumn(t.table, "user_id") {
				if err := dropUserIDColumn(tx, t.model, t.table, t.foreignKey); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// dropUserIDColumn removes user_id from one table. SQLite cannot drop a column that is part of
// a foreign key or index, so those go first; the constraint drop rebuilds the table.
func dropUserIDColumn(tx *gorm.DB, model any, table, foreignKey string) error {
	m := tx.Migrator()
	if foreignKey != "" && m.HasConstraint(model, foreignKey) {
		if err := m.DropConstraint(model, foreignKey); err != nil {
			return fmt.Errorf("failed to drop %s: %w", foreignKey, err)
		}
	}
	for _, index := range []string{"idx_" + table + "_user_id", "idx_tag_user_name"} {
		if err := tx.Exec("DROP INDEX IF EXISTS `" + index + "`").Error; err != nil {
			return fmt.Errorf("failed to drop index %s: %w", index, err)
		}
	}
	if err := tx.Exec("ALTER TABLE `" + table + "` DROP COLUMN `user_id`").Error; err != nil {
		return fmt.Errorf("failed to drop %s.user_id: %w", table, err)
	}
	return nil
}

// mergeDuplicateTags folds tags whose names differ only by case (or by legacy owner) into the oldest one.
func mergeDuplicateTags(tx *gorm.DB) error {
	const duplicates = `SELECT id, (SELECT MIN(k.id) FROM tags k WHERE LOWER(k.name) = LOWER(t.name)) AS keep_id
		FROM tags t WHERE id <> (SELECT MIN(k.id) FROM tags k WHERE LOWER(k.name) = LOWER(t.name))`
	queries := []string{
		`INSERT OR IGNORE INTO book_tags (tag_id, book_id)
			SELECT d.keep_id, bt.book_id FROM book_tags bt JOIN (` + duplicates + `) d ON bt.tag_id = d.id`,
		`INSERT OR IGNORE INTO highlight_tags (tag_id, highlight_id)
			SELECT d.keep_id, ht.highlight_id FROM highlight_tags ht JOIN (` + duplicates + `) d ON ht.tag_id = d.id`,
		`DELETE FROM book_tags WHERE tag_id IN (SELECT id FROM (` + duplicates + `))`,
		`DELETE FROM highlight_tags WHERE tag_id IN (SELECT id FROM (` + duplicates + `))`,
		`DELETE FROM tags WHERE id IN (SELECT id FROM (` + duplicates + `))`,
	}
	for _, q := range queries {
		if err := tx.Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}
