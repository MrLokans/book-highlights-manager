package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createLegacyDB builds a database with the multi-user schema and some data spread over user 0 and user 1.
func createLegacyDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	schema, err := os.ReadFile("testdata/legacy_multiuser_schema.sql")
	require.NoError(t, err)

	raw, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer raw.Close()

	_, err = raw.Exec(string(schema))
	require.NoError(t, err)
	_, err = raw.Exec(`
		INSERT INTO users (id, username, email, role) VALUES (1, 'owner', 'owner@example.com', 'admin');
		INSERT INTO books (id, user_id, title, author, cover_url) VALUES (1, 1, 'Imported', 'A', 'http://cover'), (2, 0, 'Synced', 'B', '');
		INSERT INTO highlights (id, book_id, user_id, text, is_favorite) VALUES (1, 1, 1, 'h1', true), (2, 2, 0, 'h2', false);
		INSERT INTO tags (id, user_id, name) VALUES (1, 1, 'Fiction'), (2, 0, 'fiction'), (3, 1, 'history');
		INSERT INTO book_tags (tag_id, book_id) VALUES (1, 1), (2, 1), (2, 2);
		INSERT INTO highlight_tags (tag_id, highlight_id) VALUES (2, 2), (3, 1);
		INSERT INTO words (id, user_id, word, book_id, highlight_id) VALUES (1, 1, 'ephemeral', 1, 1);
		INSERT INTO deleted_entities (id, user_id, entity_type, entity_key) VALUES (1, 1, 'book', 'Gone|X');
		INSERT INTO import_sessions (id, user_id, status) VALUES (1, 1, 'completed');
	`)
	require.NoError(t, err)
	return path
}

func TestNewDatabase_DropsLegacyUserColumns(t *testing.T) {
	path := createLegacyDB(t)

	db, err := NewDatabase(path)
	require.NoError(t, err)
	defer db.Close()

	m := db.DB.Migrator()
	for _, table := range []string{"books", "highlights", "tags", "import_sessions", "words", "deleted_entities"} {
		assert.False(t, m.HasColumn(table, "user_id"), "%s.user_id should be gone", table)
	}
	assert.True(t, m.HasIndex("books", "idx_books_title"), "indexes are recreated after the table rebuild")
	assert.FileExists(t, path+".pre-user-id-migration.bak")

	var count int64
	db.DB.Table("books").Count(&count)
	assert.Equal(t, int64(2), count)
	db.DB.Table("highlights").Count(&count)
	assert.Equal(t, int64(2), count)

	var cover string
	db.DB.Raw("SELECT cover_url FROM books WHERE id = 1").Scan(&cover)
	assert.Equal(t, "http://cover", cover)

	var favourite bool
	db.DB.Raw("SELECT is_favorite FROM highlights WHERE id = 1").Scan(&favourite)
	assert.True(t, favourite)

	var wordBookID uint
	db.DB.Raw("SELECT book_id FROM words WHERE id = 1").Scan(&wordBookID)
	assert.Equal(t, uint(1), wordBookID)

	// "Fiction" and "fiction" merge into the oldest tag, keeping every link.
	var tagNames []string
	db.DB.Raw("SELECT name FROM tags ORDER BY id").Scan(&tagNames)
	assert.Equal(t, []string{"Fiction", "history"}, tagNames)

	var bookLinks []string
	db.DB.Raw("SELECT tag_id || ':' || book_id FROM book_tags ORDER BY tag_id, book_id").Scan(&bookLinks)
	assert.Equal(t, []string{"1:1", "1:2"}, bookLinks)

	var highlightLinks []string
	db.DB.Raw("SELECT tag_id || ':' || highlight_id FROM highlight_tags ORDER BY tag_id, highlight_id").Scan(&highlightLinks)
	assert.Equal(t, []string{"1:2", "3:1"}, highlightLinks)

	err = db.DB.Exec("INSERT INTO tags (name) VALUES ('history')").Error
	require.Error(t, err, "tag names are unique now")
	assert.True(t, strings.Contains(err.Error(), "UNIQUE"))
}

func TestNewDatabase_LegacyMigrationRunsOnce(t *testing.T) {
	path := createLegacyDB(t)

	db, err := NewDatabase(path)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	require.NoError(t, os.Remove(path+".pre-user-id-migration.bak"))

	db, err = NewDatabase(path)
	require.NoError(t, err)
	defer db.Close()
	assert.NoFileExists(t, path+".pre-user-id-migration.bak", "an already migrated database is left alone")
}

func TestNewDatabase_FreshDatabaseHasNoUserColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")

	db, err := NewDatabase(path)
	require.NoError(t, err)
	defer db.Close()

	assert.False(t, db.DB.Migrator().HasColumn("books", "user_id"))
	assert.NoFileExists(t, path+".pre-user-id-migration.bak")
}
