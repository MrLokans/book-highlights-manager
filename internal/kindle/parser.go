package kindle

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

// Entry types in Kindle clippings
type EntryType string

const (
	EntryTypeHighlight EntryType = "highlight"
	EntryTypeNote      EntryType = "note"
	EntryTypeBookmark  EntryType = "bookmark"
)

// ClippingEntry represents a single parsed entry from My Clippings.txt
type ClippingEntry struct {
	Title         string
	Author        string
	Type          EntryType
	Page          int
	PageEnd       int
	Location      int
	LocationEnd   int
	AddedAt       time.Time
	Text          string
}

// Parser parses Kindle My Clippings.txt format
type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

const entrySeparator = "=========="

// Regex patterns for parsing metadata lines
var (
	// Matches: "- Your Highlight on page 8 | Location 64-64 | Added on Tuesday, April 15, 2025 10:16:21 PM"
	// or: "- Your Note on page 31 | Location 307 | Added on Tuesday, April 15, 2025 11:33:26 PM"
	// or: "- Your Highlight at location 784-785 | Added on Saturday, 26 March 2016 18:37:26"
	// or: "- Your Bookmark at location 346 | Added on Saturday, 26 March 2016 15:46:21"
	metadataPattern = regexp.MustCompile(`^- Your (Highlight|Note|Bookmark)`)

	// Page patterns: "on page 8" or "on page 207-207"
	pagePattern = regexp.MustCompile(`(?i)(?:on )?page (\d+)(?:-(\d+))?`)

	// Location patterns: "Location 64-64" or "location 1406-1407" or "at location 784-785"
	locationPattern = regexp.MustCompile(`(?i)(?:at )?location (\d+)(?:-(\d+))?`)

	// Date patterns - multiple formats observed in the wild
	// "Added on Tuesday, April 15, 2025 10:16:21 PM"
	// "Added on Saturday, 26 March 2016 14:59:39"
	datePatterns = []string{
		"Added on Monday, January 2, 2006 3:04:05 PM",
		"Added on Monday, January 2, 2006 15:04:05",
		"Added on Monday, 2 January 2006 3:04:05 PM",
		"Added on Monday, 2 January 2006 15:04:05",
	}

	// Title with author: "Book Title (Author Name)"
	// Some books don't have author in parentheses
	titleAuthorPattern = regexp.MustCompile(`^(.+?)\s*\(([^)]+)\)\s*$`)
)

// Parse reads a Kindle My Clippings.txt file and returns parsed books with highlights
func (p *Parser) Parse(r io.Reader) ([]entities.Book, error) {
	entries, err := p.ParseEntries(r)
	if err != nil {
		return nil, err
	}

	return p.groupEntriesIntoBooks(entries), nil
}

// ParseEntries parses individual clipping entries from the reader
func (p *Parser) ParseEntries(r io.Reader) ([]ClippingEntry, error) {
	scanner := bufio.NewScanner(r)

	var entries []ClippingEntry
	var currentLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == entrySeparator {
			if len(currentLines) > 0 {
				entry, err := p.parseEntry(currentLines)
				if err == nil && entry != nil {
					entries = append(entries, *entry)
				}
				currentLines = nil
			}
			continue
		}

		currentLines = append(currentLines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading clippings: %w", err)
	}

	// Handle last entry if file doesn't end with separator
	if len(currentLines) > 0 {
		entry, err := p.parseEntry(currentLines)
		if err == nil && entry != nil {
			entries = append(entries, *entry)
		}
	}

	return entries, nil
}

func (p *Parser) parseEntry(lines []string) (*ClippingEntry, error) {
	if len(lines) < 2 {
		return nil, fmt.Errorf("entry too short")
	}

	// First line: Title (Author) or just Title
	titleLine := strings.TrimSpace(lines[0])
	if titleLine == "" {
		return nil, fmt.Errorf("empty title line")
	}

	title, author := parseTitleAuthor(titleLine)

	// Second line: Metadata (type, page, location, date)
	metadataLine := strings.TrimSpace(lines[1])
	if !metadataPattern.MatchString(metadataLine) {
		return nil, fmt.Errorf("invalid metadata line")
	}

	entryType := parseEntryType(metadataLine)
	page, pageEnd := parsePageRange(metadataLine)
	location, locationEnd := parseLocationRange(metadataLine)
	addedAt := parseDate(metadataLine)

	// Remaining lines (after blank line): Text content
	// Format is: title, metadata, blank line, content
	var textLines []string
	startContent := false
	for i := 2; i < len(lines); i++ {
		line := lines[i]
		if !startContent && strings.TrimSpace(line) == "" {
			startContent = true
			continue
		}
		if startContent || strings.TrimSpace(line) != "" {
			startContent = true
			textLines = append(textLines, line)
		}
	}

	text := strings.TrimSpace(strings.Join(textLines, "\n"))

	// Bookmarks are skipped entirely (they have no text content)
	if entryType == EntryTypeBookmark {
		return nil, fmt.Errorf("bookmark entry")
	}

	// Highlights and notes should have text
	if text == "" {
		return nil, fmt.Errorf("empty content")
	}

	return &ClippingEntry{
		Title:       title,
		Author:      author,
		Type:        entryType,
		Page:        page,
		PageEnd:     pageEnd,
		Location:    location,
		LocationEnd: locationEnd,
		AddedAt:     addedAt,
		Text:        text,
	}, nil
}

func parseTitleAuthor(line string) (title, author string) {
	matches := titleAuthorPattern.FindStringSubmatch(line)
	if len(matches) == 3 {
		return strings.TrimSpace(matches[1]), strings.TrimSpace(matches[2])
	}
	// No author in parentheses, use whole line as title
	return strings.TrimSpace(line), ""
}

func parseEntryType(line string) EntryType {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "your highlight"):
		return EntryTypeHighlight
	case strings.Contains(lower, "your note"):
		return EntryTypeNote
	case strings.Contains(lower, "your bookmark"):
		return EntryTypeBookmark
	default:
		return EntryTypeHighlight
	}
}

func parsePageRange(line string) (page, pageEnd int) {
	matches := pagePattern.FindStringSubmatch(line)
	if len(matches) >= 2 {
		page, _ = strconv.Atoi(matches[1])
		if len(matches) >= 3 && matches[2] != "" {
			pageEnd, _ = strconv.Atoi(matches[2])
		}
	}
	return
}

func parseLocationRange(line string) (location, locationEnd int) {
	matches := locationPattern.FindStringSubmatch(line)
	if len(matches) >= 2 {
		location, _ = strconv.Atoi(matches[1])
		if len(matches) >= 3 && matches[2] != "" {
			locationEnd, _ = strconv.Atoi(matches[2])
		}
	}
	return
}

func parseDate(line string) time.Time {
	// Extract the date part after "Added on"
	idx := strings.Index(strings.ToLower(line), "added on")
	if idx == -1 {
		return time.Time{}
	}

	dateStr := "Added on" + line[idx+8:]
	dateStr = strings.TrimSpace(dateStr)

	for _, pattern := range datePatterns {
		t, err := time.Parse(pattern, dateStr)
		if err == nil {
			return t
		}
	}

	return time.Time{}
}

func (p *Parser) groupEntriesIntoBooks(entries []ClippingEntry) []entities.Book {
	// Group entries by book (title + author combination)
	bookMap := make(map[string]*entities.Book)
	bookOrder := []string{}

	// Track notes that should be attached to highlights
	// Notes typically appear after highlights at the same location
	notesByBook := make(map[string][]ClippingEntry)

	// First pass: separate highlights and notes
	for _, entry := range entries {
		key := bookKey(entry.Title, entry.Author)

		if entry.Type == EntryTypeNote {
			notesByBook[key] = append(notesByBook[key], entry)
			continue
		}

		if entry.Type == EntryTypeBookmark {
			// Skip bookmarks for now
			continue
		}

		// Process highlights
		book, exists := bookMap[key]
		if !exists {
			book = &entities.Book{
				Title:  entry.Title,
				Author: entry.Author,
				Source: entities.Source{
					Name:        "kindle",
					DisplayName: "Amazon Kindle",
				},
				Highlights: []entities.Highlight{},
			}
			bookMap[key] = book
			bookOrder = append(bookOrder, key)
		}

		highlight := p.entryToHighlight(entry)
		book.Highlights = append(book.Highlights, highlight)
	}

	// Second pass: try to attach notes to their corresponding highlights
	for bookKey, notes := range notesByBook {
		book, exists := bookMap[bookKey]
		if !exists {
			// Create book for standalone notes
			if len(notes) > 0 {
				firstNote := notes[0]
				book = &entities.Book{
					Title:  firstNote.Title,
					Author: firstNote.Author,
					Source: entities.Source{
						Name:        "kindle",
						DisplayName: "Amazon Kindle",
					},
					Highlights: []entities.Highlight{},
				}
				bookMap[bookKey] = book
				bookOrder = append(bookOrder, bookKey)
			}
		}

		for _, note := range notes {
			attached := false

			// Try to find a highlight at the same location to attach the note
			for i := range book.Highlights {
				h := &book.Highlights[i]
				if matchesLocation(h, note) {
					// Attach note to existing highlight
					if h.Note == "" {
						h.Note = note.Text
					} else {
						h.Note = h.Note + "\n\n" + note.Text
					}
					attached = true
					break
				}
			}

			// If no matching highlight, create a note-only highlight
			if !attached {
				highlight := entities.Highlight{
					Note:          note.Text,
					LocationType:  entities.LocationTypeLocation,
					LocationValue: note.Location,
					LocationEnd:   note.LocationEnd,
					HighlightedAt: note.AddedAt,
					Style:         entities.HighlightStyleNoteOnly,
					ExternalID:    generateExternalID(note),
					Source: entities.Source{
						Name:        "kindle",
						DisplayName: "Amazon Kindle",
					},
				}
				if note.Page > 0 {
					highlight.LocationType = entities.LocationTypePage
					highlight.LocationValue = note.Page
					highlight.LocationEnd = note.PageEnd
				}
				book.Highlights = append(book.Highlights, highlight)
			}
		}
	}

	// Convert to slice in original order
	var books []entities.Book
	for _, key := range bookOrder {
		book := bookMap[key]
		if len(book.Highlights) > 0 {
			books = append(books, *book)
		}
	}

	return books
}

func (p *Parser) entryToHighlight(entry ClippingEntry) entities.Highlight {
	highlight := entities.Highlight{
		Text:          entry.Text,
		HighlightedAt: entry.AddedAt,
		Style:         entities.HighlightStyleHighlight,
		ExternalID:    generateExternalID(entry),
		Source: entities.Source{
			Name:        "kindle",
			DisplayName: "Amazon Kindle",
		},
	}

	// Prefer location over page for Kindle
	if entry.Location > 0 {
		highlight.LocationType = entities.LocationTypeLocation
		highlight.LocationValue = entry.Location
		highlight.LocationEnd = entry.LocationEnd
	} else if entry.Page > 0 {
		highlight.LocationType = entities.LocationTypePage
		highlight.LocationValue = entry.Page
		highlight.LocationEnd = entry.PageEnd
	}

	return highlight
}

func bookKey(title, author string) string {
	return strings.ToLower(title) + "|" + strings.ToLower(author)
}

func matchesLocation(h *entities.Highlight, note ClippingEntry) bool {
	// Check if note is at same location as highlight
	if note.Location > 0 && h.LocationType == entities.LocationTypeLocation {
		return h.LocationValue == note.Location
	}
	if note.Page > 0 && h.LocationType == entities.LocationTypePage {
		return h.LocationValue == note.Page
	}
	return false
}

func generateExternalID(entry ClippingEntry) string {
	// Generate a unique ID based on book, location, and timestamp
	loc := entry.Location
	if loc == 0 {
		loc = entry.Page
	}
	return fmt.Sprintf("kindle-%s-%d-%d", sanitizeForID(entry.Title), loc, entry.AddedAt.Unix())
}

func sanitizeForID(s string) string {
	// Keep only alphanumeric characters and convert to lowercase
	var result strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		}
	}
	return result.String()
}
