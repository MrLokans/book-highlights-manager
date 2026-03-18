package moonreader

import (
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertNoteToHighlight(t *testing.T) {
	t.Run("converts basic fields", func(t *testing.T) {
		now := time.Now()
		note := &LocalNote{
			Original:   "highlighted text",
			NoteText:   "my note",
			Time:       now,
			Bookmark:   "Chapter 1",
			ExternalID: "ext-123",
			Color:      "-256",
		}

		h := ConvertNoteToHighlight(note)

		assert.Equal(t, "highlighted text", h.Text)
		assert.Equal(t, "my note", h.Note)
		assert.Equal(t, now, h.HighlightedAt)
		assert.Equal(t, "Chapter 1", h.Chapter)
		assert.Equal(t, "ext-123", h.ExternalID)
		assert.Equal(t, entities.HighlightStyleHighlight, h.Style)
	})

	t.Run("sets underline style", func(t *testing.T) {
		note := &LocalNote{Underline: true}
		h := ConvertNoteToHighlight(note)
		assert.Equal(t, entities.HighlightStyleUnderline, h.Style)
	})

	t.Run("sets strikethrough style", func(t *testing.T) {
		note := &LocalNote{Strikethrough: true}
		h := ConvertNoteToHighlight(note)
		assert.Equal(t, entities.HighlightStyleStrikethrough, h.Style)
	})
}

func TestConvertToEntities(t *testing.T) {
	t.Run("converts grouped notes to books", func(t *testing.T) {
		notesByBook := map[string][]*LocalNote{
			"Book A": {
				{Original: "h1", Filename: "Book A - Author.epub", BookTitle: "Book A"},
				{Original: "h2", Filename: "Book A - Author.epub", BookTitle: "Book A"},
			},
			"Book B": {
				{Original: "h3", Filename: "Book B.epub", BookTitle: "Book B"},
			},
		}

		books := ConvertToEntities(notesByBook)

		require.Len(t, books, 2)

		bookMap := make(map[string]entities.Book)
		for _, b := range books {
			bookMap[b.Title] = b
		}

		assert.Len(t, bookMap["Book A"].Highlights, 2)
		assert.Len(t, bookMap["Book B"].Highlights, 1)
		assert.Equal(t, "moonreader", bookMap["Book A"].Source.Name)
	})

	t.Run("skips books with no notes", func(t *testing.T) {
		notesByBook := map[string][]*LocalNote{
			"Empty Book": {},
		}

		books := ConvertToEntities(notesByBook)
		assert.Len(t, books, 0)
	})

	t.Run("returns empty for nil input", func(t *testing.T) {
		books := ConvertToEntities(map[string][]*LocalNote{})
		assert.Len(t, books, 0)
	})
}
