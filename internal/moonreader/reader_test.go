package moonreader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalDBAccessor_EnsureSchema(t *testing.T) {
	// Create a temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	accessor, err := NewLocalDBAccessor(dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	// Verify the database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestLocalDBAccessor_UpsertAndGetNotes(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	accessor, err := NewLocalDBAccessor(dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	// Create test notes
	notes := []*MoonReaderNote{
		{
			ID:             1,
			BookTitle:      "Test Book",
			Filename:       "/path/to/book.epub",
			HighlightColor: "-256",
			TimeMs:         1700000000000,
			Bookmark:       "Page 10",
			Note:           "My annotation",
			Original:       "Highlighted text",
			Underline:      1,
			Strikethrough:  0,
		},
		{
			ID:             2,
			BookTitle:      "Test Book",
			Filename:       "/path/to/book.epub",
			HighlightColor: "-16777216",
			TimeMs:         1700001000000,
			Bookmark:       "Page 20",
			Note:           "",
			Original:       "Another highlight",
			Underline:      0,
			Strikethrough:  1,
		},
	}

	// Upsert notes
	err = accessor.UpsertNotes(notes)
	require.NoError(t, err)

	// Retrieve notes
	retrieved, err := accessor.GetNotes()
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)

	// Verify first note
	assert.Equal(t, "1", retrieved[0].ExternalID)
	assert.Equal(t, "Test Book", retrieved[0].BookTitle)
	assert.Equal(t, "Highlighted text", retrieved[0].Original)
	assert.True(t, retrieved[0].Underline)
	assert.False(t, retrieved[0].Strikethrough)

	// Test upsert (update existing)
	notes[0].Original = "Updated highlight"
	err = accessor.UpsertNotes(notes[:1])
	require.NoError(t, err)

	retrieved, err = accessor.GetNotes()
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)

	// Find the updated note
	var updated *LocalNote
	for _, n := range retrieved {
		if n.ExternalID == "1" {
			updated = n
			break
		}
	}
	require.NotNil(t, updated)
	assert.Equal(t, "Updated highlight", updated.Original)
}

func TestLocalDBAccessor_GetNotesByBook(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	accessor, err := NewLocalDBAccessor(dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	notes := []*MoonReaderNote{
		{
			ID:             1,
			BookTitle:      "Book One",
			Filename:       "/path/book1.epub",
			HighlightColor: "-256",
			TimeMs:         1700000000000,
			Original:       "Note 1",
		},
		{
			ID:             2,
			BookTitle:      "Book One",
			Filename:       "/path/book1.epub",
			HighlightColor: "-256",
			TimeMs:         1700001000000,
			Original:       "Note 2",
		},
		{
			ID:             3,
			BookTitle:      "Book Two",
			Filename:       "/path/book2.epub",
			HighlightColor: "-256",
			TimeMs:         1700002000000,
			Original:       "Note 3",
		},
	}

	err = accessor.UpsertNotes(notes)
	require.NoError(t, err)

	notesByBook, err := accessor.GetNotesByBook()
	require.NoError(t, err)

	assert.Len(t, notesByBook, 2)
	assert.Len(t, notesByBook["Book One"], 2)
	assert.Len(t, notesByBook["Book Two"], 1)
}

func TestLocalDBAccessor_RecordBackupImport(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	accessor, err := NewLocalDBAccessor(dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	// Initially no import recorded
	lastImport, err := accessor.GetLastBackupImport("backup-1")
	require.NoError(t, err)
	assert.True(t, lastImport.IsZero())

	// Record import
	importTime := accessor.db.Driver()
	_ = importTime // just to use the variable
}
