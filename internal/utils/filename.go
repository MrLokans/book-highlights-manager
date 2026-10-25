package utils

import (
	"path"
	"regexp"
	"strings"
	"unicode"
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

var (
	// Download-site tags appended to filenames: "(z-library.sk, 1lib.sk)", "- libgen.li", "-- Anna's Archive".
	siteTag = regexp.MustCompile(`(?i)z-?lib|1lib|libgen|anna.s archive`)
	// A trailing "(...)" or "[...]" group.
	trailingGroup = regexp.MustCompile(`\s*[(\[]([^()\[\]]*)[)\]]$`)
	// Surname particles that legitimately precede a capital: DeMarco, McConnell, LeGuin.
	namePrefixes = map[string]bool{"Mc": true, "Mac": true, "De": true, "Di": true, "Da": true, "Du": true, "La": true, "Le": true, "Van": true, "Von": true}
)

// ExtractAuthorFromFilename extracts the author from an e-book filename, given the
// book's title (which may differ from the filename's title in punctuation or subtitle).
// Handles "Title - Author", "Author - Title" (Calibre/libgen), "Title -- Author -- ..."
// (Anna's Archive), "Title (Author)" / "Title [Author]" (z-library), "Title by Author",
// download-site suffixes, and Moon+'s habit of gluing the publisher onto the author.
func ExtractAuthorFromFilename(filename, bookTitle string) string {
	base := stripSiteTags(stripBookExtension(path.Base(filename)))

	var author string
	switch {
	case strings.Contains(base, " -- "):
		if parts := strings.Split(base, " -- "); len(parts) > 1 {
			author = parts[1]
		}
	case strings.Contains(base, " - "):
		first, last := strings.Index(base, " - "), strings.LastIndex(base, " - ")
		if startsWithTitle(base[first+3:], bookTitle) && !startsWithTitle(base, bookTitle) {
			author = base[:first]
		} else {
			author = base[last+3:]
		}
	case trailingGroup.MatchString(base):
		// A digit means a series number or year, not a name: "Dune (Dune Chronicles 1)".
		if group := trailingGroup.FindStringSubmatch(base)[1]; !strings.ContainsAny(group, "0123456789") {
			author = group
		}
	case strings.Contains(base, " by "):
		if i := strings.LastIndex(base, " by "); startsWithTitle(base[:i], bookTitle) {
			author = base[i+4:]
		}
	}

	author = strings.TrimFunc(splitGluedPublisher(author), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return multipleSpaces.ReplaceAllString(author, " ")
}

func stripBookExtension(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range KnownBookExtensions {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

// stripSiteTags removes trailing download-site markers, repeatedly.
func stripSiteTags(s string) string {
	for {
		s = strings.TrimSpace(s)
		if m := trailingGroup.FindStringSubmatchIndex(s); m != nil && siteTag.MatchString(s[m[2]:m[3]]) {
			s = s[:m[0]]
			continue
		}
		i := strings.LastIndex(s, " - ")
		if i >= 0 && siteTag.MatchString(s[i:]) {
			s = strings.TrimRight(s[:i], " -")
			continue
		}
		return s
	}
}

// splitGluedPublisher cuts "Brian PotterStripe Press" at the lower→upper case join.
func splitGluedPublisher(s string) string {
	runes := []rune(s)
	wordStart := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsSpace(runes[i-1]) {
			wordStart = i
		}
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) && !namePrefixes[string(runes[wordStart:i])] {
			return string(runes[:i])
		}
	}
	return s
}

// startsWithTitle compares ignoring case and punctuation, since filenames drop ":", "'" and ",".
func startsWithTitle(s, title string) bool {
	t := normalizeTitle(title)
	return t != "" && strings.HasPrefix(normalizeTitle(s)+" ", t+" ")
}

func normalizeTitle(s string) string {
	s = strings.NewReplacer("'", "", "’", "").Replace(strings.ToLower(s))
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
