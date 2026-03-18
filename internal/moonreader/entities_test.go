package moonreader

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNote_GetTime(t *testing.T) {
	note := &Note{TimeMs: 1705320000000}
	ts := note.GetTime()
	assert.Equal(t, 2024, ts.Year())
}

func TestNote_GetColorHex(t *testing.T) {
	t.Run("converts valid color", func(t *testing.T) {
		note := &Note{HighlightColor: "-256"}
		assert.Equal(t, "#FFFFFF00", note.GetColorHex())
	})
	t.Run("returns default for invalid", func(t *testing.T) {
		note := &Note{HighlightColor: "not-a-number"}
		assert.Equal(t, "#FFFFFF00", note.GetColorHex())
	})
}

func TestNote_IsUnderlined(t *testing.T) {
	assert.True(t, (&Note{Underline: 1}).IsUnderlined())
	assert.False(t, (&Note{Underline: 0}).IsUnderlined())
}

func TestNote_IsStrikethrough(t *testing.T) {
	assert.True(t, (&Note{Strikethrough: 1}).IsStrikethrough())
	assert.False(t, (&Note{Strikethrough: 0}).IsStrikethrough())
}

func TestNote_GetText(t *testing.T) {
	assert.Equal(t, "original", (&Note{Original: "original", Note: "note"}).GetText())
	assert.Equal(t, "note", (&Note{Note: "note"}).GetText())
}

func TestNote_GetAuthor(t *testing.T) {
	note := &Note{Filename: "My Book - John Doe.epub", BookTitle: "My Book"}
	assert.Equal(t, "John Doe", note.GetAuthor())
}

func TestLocalNote_GetText(t *testing.T) {
	assert.Equal(t, "original", (&LocalNote{Original: "original", NoteText: "note"}).GetText())
	assert.Equal(t, "note", (&LocalNote{NoteText: "note"}).GetText())
}

func TestLocalNote_GetColorHex(t *testing.T) {
	assert.Equal(t, "#FFFFFF00", (&LocalNote{Color: "-256"}).GetColorHex())
	assert.Equal(t, "#FFFFFF00", (&LocalNote{Color: "invalid"}).GetColorHex())
}

func TestLocalNote_GetAuthor(t *testing.T) {
	note := &LocalNote{Filename: "Title - Author.epub", BookTitle: "Title"}
	assert.Equal(t, "Author", note.GetAuthor())
}
