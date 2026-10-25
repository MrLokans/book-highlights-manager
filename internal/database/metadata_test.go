package database

import (
	"errors"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- MetadataUpdater mocks ---

type mockBooksMetadataReader struct {
	book          *entities.Book
	getErr        error
	updateErr     error
	updatedFields map[string]any
	missingBooks  []entities.Book
	missingErr    error
}

func (m *mockBooksMetadataReader) GetBookByID(_ uint) (*entities.Book, error) {
	return m.book, m.getErr
}

func (m *mockBooksMetadataReader) UpdateBookMetadata(_ uint, fields map[string]any) error {
	m.updatedFields = fields
	return m.updateErr
}

func (m *mockBooksMetadataReader) GetBooksMissingMetadata() ([]entities.Book, error) {
	return m.missingBooks, m.missingErr
}

func TestMetadataUpdater_GetBookByID(t *testing.T) {
	book := &entities.Book{Title: "Test"}
	mock := &mockBooksMetadataReader{book: book}
	updater := NewMetadataUpdater(mock)

	result, err := updater.GetBookByID(1)
	require.NoError(t, err)
	assert.Equal(t, "Test", result.Title)
}

func TestMetadataUpdater_GetBookByID_Error(t *testing.T) {
	mock := &mockBooksMetadataReader{getErr: errors.New("not found")}
	updater := NewMetadataUpdater(mock)

	_, err := updater.GetBookByID(1)
	assert.Error(t, err)
}

func TestMetadataUpdater_UpdateBookMetadata(t *testing.T) {
	t.Run("converts all fields to map", func(t *testing.T) {
		mock := &mockBooksMetadataReader{}
		updater := NewMetadataUpdater(mock)

		isbn := "978-0-13-468599-1"
		coverURL := "https://example.com/cover.jpg"
		publisher := "Addison-Wesley"
		year := 2020

		err := updater.UpdateBookMetadata(1, metadata.BookUpdateFields{
			ISBN:            &isbn,
			CoverURL:        &coverURL,
			Publisher:       &publisher,
			PublicationYear: &year,
		})
		require.NoError(t, err)
		assert.Equal(t, isbn, mock.updatedFields["isbn"])
		assert.Equal(t, coverURL, mock.updatedFields["cover_url"])
		assert.Equal(t, publisher, mock.updatedFields["publisher"])
		assert.Equal(t, year, mock.updatedFields["publication_year"])
	})

	t.Run("skips nil fields", func(t *testing.T) {
		mock := &mockBooksMetadataReader{}
		updater := NewMetadataUpdater(mock)

		isbn := "978-0-13-468599-1"
		err := updater.UpdateBookMetadata(1, metadata.BookUpdateFields{
			ISBN: &isbn,
		})
		require.NoError(t, err)
		assert.Len(t, mock.updatedFields, 1)
		assert.Equal(t, isbn, mock.updatedFields["isbn"])
	})

	t.Run("no-op when all fields nil", func(t *testing.T) {
		mock := &mockBooksMetadataReader{}
		updater := NewMetadataUpdater(mock)

		err := updater.UpdateBookMetadata(1, metadata.BookUpdateFields{})
		require.NoError(t, err)
		assert.Nil(t, mock.updatedFields)
	})

	t.Run("propagates error", func(t *testing.T) {
		mock := &mockBooksMetadataReader{updateErr: errors.New("db error")}
		updater := NewMetadataUpdater(mock)

		isbn := "test"
		err := updater.UpdateBookMetadata(1, metadata.BookUpdateFields{ISBN: &isbn})
		assert.Error(t, err)
	})
}

func TestMetadataUpdater_GetBooksMissingMetadata(t *testing.T) {
	books := []entities.Book{{Title: "No Cover"}}
	mock := &mockBooksMetadataReader{missingBooks: books}
	updater := NewMetadataUpdater(mock)

	result, err := updater.GetBooksMissingMetadata()
	require.NoError(t, err)
	assert.Len(t, result, 1)
}
