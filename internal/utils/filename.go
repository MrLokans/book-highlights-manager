package utils

import (
	"regexp"
	"strings"
)

var (
	// Characters invalid in filenames on most filesystems
	invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*]`)
	// Whitespace characters to normalize
	whitespaceChars = regexp.MustCompile(`[\r\n\t]`)
	// Multiple spaces to collapse
	multipleSpaces = regexp.MustCompile(`\s+`)
)

// SanitizeFilename sanitizes a filename for Obsidian compatibility.
// It removes or replaces characters that are invalid in filenames or
// problematic in Obsidian (slashes, colons, quotes, hashtags, brackets, etc.)
func SanitizeFilename(filename string) string {
	// Remove invalid filename characters
	filename = invalidFilenameChars.ReplaceAllString(filename, "")

	// Replace newlines/tabs with spaces
	filename = whitespaceChars.ReplaceAllString(filename, " ")

	// Collapse multiple spaces
	filename = multipleSpaces.ReplaceAllString(filename, " ")

	// Trim whitespace
	filename = strings.TrimSpace(filename)

	// Obsidian-specific sanitization
	filename = strings.ReplaceAll(filename, "#", "")
	filename = strings.ReplaceAll(filename, "[", "(")
	filename = strings.ReplaceAll(filename, "]", ")")

	// Limit length (most filesystems support 255, but leave room for extension)
	if len(filename) > 200 {
		filename = strings.TrimSpace(filename[:200])
	}

	// Ensure it's not empty
	if filename == "" {
		filename = "Untitled"
	}

	return filename
}

// KnownBookExtensions contains file extensions commonly used for e-books
var KnownBookExtensions = []string{
	".fb2.zip",
	".fb2",
	".epub",
	".pdf",
	".txt",
	".tar.gz",
	".docx",
	".doc",
	".mobi",
	".azw3",
	".azw",
	".djvu",
}

// ExtractAuthorFromFilename attempts to extract an author name from a MoonReader filename.
// MoonReader typically stores files as "Title - Author.extension"
func ExtractAuthorFromFilename(filename, bookTitle string) string {
	// Find where the title appears in the filename
	titlePos := strings.LastIndex(filename, bookTitle)
	if titlePos == -1 {
		return ""
	}

	// Get everything after the title
	possibleAuthor := filename[titlePos+len(bookTitle):]

	// Remove known extensions
	for _, ext := range KnownBookExtensions {
		possibleAuthor = strings.TrimSuffix(possibleAuthor, ext)
	}

	// Clean up non-alphanumeric characters from beginning and end
	possibleAuthor = strings.TrimFunc(possibleAuthor, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r >= 0x80) // Keep unicode letters
	})

	// Also try common separators
	possibleAuthor = strings.TrimPrefix(possibleAuthor, " - ")
	possibleAuthor = strings.TrimPrefix(possibleAuthor, "-")
	possibleAuthor = strings.TrimSpace(possibleAuthor)

	return possibleAuthor
}
