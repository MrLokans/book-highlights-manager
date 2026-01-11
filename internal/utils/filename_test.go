package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes invalid characters",
			input:    `file<>:"/\|?*name`,
			expected: "filename",
		},
		{
			name:     "replaces newlines and tabs with spaces",
			input:    "file\nname\twith\rspaces",
			expected: "file name with spaces",
		},
		{
			name:     "collapses multiple spaces",
			input:    "file   name  with    spaces",
			expected: "file name with spaces",
		},
		{
			name:     "removes hashtags",
			input:    "#hashtag #title",
			expected: "hashtag title",
		},
		{
			name:     "replaces square brackets",
			input:    "title [subtitle]",
			expected: "title (subtitle)",
		},
		{
			name:     "trims whitespace",
			input:    "  filename  ",
			expected: "filename",
		},
		{
			name:     "returns Untitled for empty",
			input:    "",
			expected: "Untitled",
		},
		{
			name:     "returns Untitled for only special chars",
			input:    "<>:?*",
			expected: "Untitled",
		},
		{
			name:     "truncates long names",
			input:    strings.Repeat("a", 250),
			expected: strings.Repeat("a", 200),
		},
		{
			name:     "handles unicode",
			input:    "Pamiętnik znaleziony w wannie",
			expected: "Pamiętnik znaleziony w wannie",
		},
		{
			name:     "complex case",
			input:    `Book: "The Title" [Vol. 1] #Series`,
			expected: "Book The Title (Vol. 1) Series",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAuthorFromFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		bookTitle string
		expected  string
	}{
		{
			name:      "extracts author from fb2 path",
			filename:  "/sdcard/Books/MoonReader/Pamiętnik znaleziony w wannie - Stanisław Lem.fb2",
			bookTitle: "Pamiętnik znaleziony w wannie",
			expected:  "Stanisław Lem",
		},
		{
			name:      "extracts author from epub",
			filename:  "/books/The Hobbit - J.R.R. Tolkien.epub",
			bookTitle: "The Hobbit",
			expected:  "J.R.R. Tolkien",
		},
		{
			name:      "handles fb2.zip extension",
			filename:  "/books/War and Peace - Leo Tolstoy.fb2.zip",
			bookTitle: "War and Peace",
			expected:  "Leo Tolstoy",
		},
		{
			name:      "returns empty if title not found",
			filename:  "/books/somefile.epub",
			bookTitle: "Different Title",
			expected:  "",
		},
		{
			name:      "handles no author",
			filename:  "/books/The Hobbit.epub",
			bookTitle: "The Hobbit",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAuthorFromFilename(tt.filename, tt.bookTitle)
			assert.Equal(t, tt.expected, result)
		})
	}
}
