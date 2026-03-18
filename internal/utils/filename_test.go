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
		{"removes invalid chars", `Book: A "Story"`, "Book A Story"},
		{"replaces newlines with spaces", "Book\nTitle", "Book Title"},
		{"replaces tabs with spaces", "Book\tTitle", "Book Title"},
		{"collapses multiple spaces", "Book   Title", "Book Title"},
		{"trims whitespace", "  Book  ", "Book"},
		{"removes hashtags", "Book #1", "Book 1"},
		{"replaces brackets", "Book [Vol 1]", "Book (Vol 1)"},
		{"removes slashes", `Book/Title\Path`, "BookTitlePath"},
		{"removes question marks", "Book?", "Book"},
		{"removes pipe", "Book|Other", "BookOther"},
		{"removes asterisks", "Book*", "Book"},
		{"returns Untitled for empty", "", "Untitled"},
		{"truncates long names", strings.Repeat("a", 250), strings.Repeat("a", 200)},
		{"passes normal filename", "Clean Book Title", "Clean Book Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeFilename(tt.input))
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
		{"extracts author with dash separator", "My Book - John Doe.epub", "My Book", "John Doe"},
		{"returns empty when title not found", "Other File.epub", "My Book", ""},
		{"handles fb2.zip extension", "My Book - Author.fb2.zip", "My Book", "Author"},
		{"handles pdf extension", "My Book - Author.pdf", "My Book", "Author"},
		{"returns empty when no author", "My Book.epub", "My Book", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ExtractAuthorFromFilename(tt.filename, tt.bookTitle))
		})
	}
}
