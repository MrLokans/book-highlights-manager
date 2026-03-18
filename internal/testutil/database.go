// Package testutil provides test helpers for database and integration tests.
package testutil

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mrlokans/assistant/internal/entities"
)

var userCounter atomic.Int64

// AllEntities returns the full list of entity types for AutoMigrate.
// Update this single list when adding new entities.
func AllEntities() []any {
	return []any{
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
	}
}

// NewTestDB creates an in-memory SQLite database with all entities migrated.
// Automatically closes the database when the test completes.
func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(AllEntities()...)
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close() //nolint:gosec // best-effort cleanup
	})

	return db
}

// NewTestDBWith creates an in-memory SQLite database migrating only the specified entities.
// Useful when you want a minimal schema for focused tests.
func NewTestDBWith(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(models...)
	require.NoError(t, err)

	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close() //nolint:gosec // best-effort cleanup
	})

	return db
}

// SeedUser creates a test user and returns it.
func SeedUser(t *testing.T, db *gorm.DB) *entities.User {
	t.Helper()
	n := userCounter.Add(1)
	user := &entities.User{
		Username: fmt.Sprintf("testuser-%d", n),
		Email:    fmt.Sprintf("testuser-%d@test.com", n),
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

// SeedSource creates a test source with the given name and returns it.
func SeedSource(t *testing.T, db *gorm.DB, name string) *entities.Source {
	t.Helper()
	source := &entities.Source{Name: name}
	require.NoError(t, db.Create(source).Error)
	return source
}

// SeedBook creates a test book with the given title and author, linked to the given source and user.
func SeedBook(t *testing.T, db *gorm.DB, title, author string, sourceID, userID uint) *entities.Book {
	t.Helper()
	book := &entities.Book{
		Title:    title,
		Author:   author,
		SourceID: sourceID,
		UserID:   userID,
	}
	require.NoError(t, db.Create(book).Error)
	return book
}

// SeedHighlight creates a test highlight for the given book.
func SeedHighlight(t *testing.T, db *gorm.DB, bookID uint, text string) *entities.Highlight {
	t.Helper()
	h := &entities.Highlight{
		BookID: bookID,
		Text:   text,
	}
	require.NoError(t, db.Create(h).Error)
	return h
}

// SeedTag creates a test tag for the given user.
func SeedTag(t *testing.T, db *gorm.DB, name string, userID uint) *entities.Tag {
	t.Helper()
	tag := &entities.Tag{Name: name, UserID: userID}
	require.NoError(t, db.Create(tag).Error)
	return tag
}
