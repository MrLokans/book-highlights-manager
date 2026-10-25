package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrlokans/assistant/internal/entities"
)

func TestNewDatabase_FixesMoonReaderAuthorsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authors.db")
	db, err := NewDatabase(path)
	require.NoError(t, err)

	var moon, kindle entities.Source
	require.NoError(t, db.DB.Where("name = ?", "moonreader").First(&moon).Error)
	require.NoError(t, db.DB.Where("name = ?", "kindle").First(&kindle).Error)
	books := []entities.Book{
		{Title: "The Origins of Efficiency", Author: "Brian PotterStripe Press", FilePath: "/sdcard/The Origins of Efficiency - Brian PotterStripe Press.epub", SourceID: moon.ID},
		{Title: "Moral Mazes: The World", Author: "", FilePath: "/sdcard/Moral Mazes The World - Robert JackallOxford University Press.pdf", SourceID: moon.ID},
		{Title: "Clash", Author: "Old AuthorPub", FilePath: "/sdcard/Clash - Old AuthorPub.epub", SourceID: moon.ID},
		{Title: "Clash", Author: "Old Author", SourceID: moon.ID},
		{Title: "Kindle Book", Author: "Some AuthorPub", FilePath: "/x/Kindle Book - Some AuthorPub.epub", SourceID: kindle.ID},
	}
	require.NoError(t, db.DB.Create(&books).Error)
	// Forget the run that happened on the empty database so the next open applies the fix.
	require.NoError(t, db.DB.Where("key = ?", moonReaderAuthorsMigrationKey).Delete(&entities.Setting{}).Error)
	require.NoError(t, db.Close())

	db, err = NewDatabase(path)
	require.NoError(t, err)
	authorOf := func(id uint) string {
		var b entities.Book
		require.NoError(t, db.DB.First(&b, id).Error)
		return b.Author
	}
	assert.Equal(t, "Brian Potter", authorOf(books[0].ID))
	assert.Equal(t, "Robert Jackall", authorOf(books[1].ID))
	assert.Equal(t, "Old AuthorPub", authorOf(books[2].ID), "left alone when the fixed author would duplicate another book")
	assert.Equal(t, "Some AuthorPub", authorOf(books[4].ID), "non-Moon+ books are untouched")

	// Runs once: a later bad author is not rewritten.
	require.NoError(t, db.DB.Model(&entities.Book{}).Where("id = ?", books[0].ID).Update("author", "Changed By User").Error)
	require.NoError(t, db.Close())
	db, err = NewDatabase(path)
	require.NoError(t, err)
	defer db.Close()
	assert.Equal(t, "Changed By User", authorOf(books[0].ID))
}
