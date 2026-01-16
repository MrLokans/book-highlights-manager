package moonreader

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMoonReaderNote_GetTime(t *testing.T) {
	note := &MoonReaderNote{
		TimeMs: 1758393655532, // 2025-09-20 20:40:55.532 UTC
	}

	result := note.GetTime()

	// The exact time depends on timezone, but we can verify it's in the right ballpark
	assert.Equal(t, 2025, result.Year())
	assert.Equal(t, time.September, result.Month())
	assert.Equal(t, 20, result.Day())
}

func TestMoonReaderNote_GetColorHex(t *testing.T) {
	tests := []struct {
		name     string
		color    string
		expected string
	}{
		{
			name:     "negative color",
			color:    "-15654349",
			expected: "#FF112233",
		},
		{
			name:     "positive color",
			color:    "1996532479",
			expected: "#7700AAFF",
		},
		{
			name:     "invalid color defaults to yellow",
			color:    "invalid",
			expected: "#FFFFFF00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := &MoonReaderNote{HighlightColor: tt.color}
			result := note.GetColorHex()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMoonReaderNote_IsUnderlined(t *testing.T) {
	tests := []struct {
		name      string
		underline int
		expected  bool
	}{
		{"zero is false", 0, false},
		{"one is true", 1, true},
		{"negative is true", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := &MoonReaderNote{Underline: tt.underline}
			assert.Equal(t, tt.expected, note.IsUnderlined())
		})
	}
}

func TestMoonReaderNote_GetText(t *testing.T) {
	tests := []struct {
		name     string
		original string
		note     string
		expected string
	}{
		{
			name:     "prefers original",
			original: "original text",
			note:     "note text",
			expected: "original text",
		},
		{
			name:     "falls back to note",
			original: "",
			note:     "note text",
			expected: "note text",
		},
		{
			name:     "empty when both empty",
			original: "",
			note:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := &MoonReaderNote{Original: tt.original, Note: tt.note}
			assert.Equal(t, tt.expected, note.GetText())
		})
	}
}

func TestMoonReaderNote_GetAuthor(t *testing.T) {
	note := &MoonReaderNote{
		BookTitle: "Pamiętnik znaleziony w wannie",
		Filename:  "/sdcard/Books/MoonReader/Pamiętnik znaleziony w wannie - Stanisław Lem.fb2",
	}

	result := note.GetAuthor()
	assert.Equal(t, "Stanisław Lem", result)
}

func TestLocalNote_GetText(t *testing.T) {
	tests := []struct {
		name     string
		original string
		noteText string
		expected string
	}{
		{
			name:     "prefers original",
			original: "original text",
			noteText: "note text",
			expected: "original text",
		},
		{
			name:     "falls back to note",
			original: "",
			noteText: "note text",
			expected: "note text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := &LocalNote{Original: tt.original, NoteText: tt.noteText}
			assert.Equal(t, tt.expected, note.GetText())
		})
	}
}
