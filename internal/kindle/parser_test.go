package kindle

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

// Test fixtures are adapted from https://github.com/biokraft/kindle2readwise/tree/main/tests/fixtures

func TestParser_ParseEntries_BasicHighlight(t *testing.T) {
	input := `The_Power_of_Now (Eckhart Tolle)
- Your Highlight on page 8 | Location 64-64 | Added on Tuesday, April 15, 2025 10:16:21 PM

would change for the better. Values would shift in the flotsam
==========
`

	parser := NewParser()
	entries, err := parser.ParseEntries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "The_Power_of_Now" {
		t.Errorf("expected title 'The_Power_of_Now', got '%s'", entry.Title)
	}
	if entry.Author != "Eckhart Tolle" {
		t.Errorf("expected author 'Eckhart Tolle', got '%s'", entry.Author)
	}
	if entry.Type != EntryTypeHighlight {
		t.Errorf("expected type highlight, got '%s'", entry.Type)
	}
	if entry.Page != 8 {
		t.Errorf("expected page 8, got %d", entry.Page)
	}
	if entry.Location != 64 {
		t.Errorf("expected location 64, got %d", entry.Location)
	}
	if entry.LocationEnd != 64 {
		t.Errorf("expected location end 64, got %d", entry.LocationEnd)
	}
	if entry.Text != "would change for the better. Values would shift in the flotsam" {
		t.Errorf("unexpected text: %s", entry.Text)
	}
}

func TestParser_ParseEntries_Note(t *testing.T) {
	input := `The_Power_of_Now (Eckhart Tolle)
- Your Note on page 31 | Location 307 | Added on Tuesday, April 15, 2025 11:33:26 PM

Watch the thinker or be present in the moment
==========
`

	parser := NewParser()
	entries, err := parser.ParseEntries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Type != EntryTypeNote {
		t.Errorf("expected type note, got '%s'", entry.Type)
	}
	if entry.Page != 31 {
		t.Errorf("expected page 31, got %d", entry.Page)
	}
	if entry.Location != 307 {
		t.Errorf("expected location 307, got %d", entry.Location)
	}
	if entry.Text != "Watch the thinker or be present in the moment" {
		t.Errorf("unexpected text: %s", entry.Text)
	}
}

func TestParser_ParseEntries_Bookmark(t *testing.T) {
	input := `Fahrenheit 451 (Ray Bradbury)
- Your Bookmark at location 346 | Added on Saturday, 26 March 2016 15:46:21


==========
`

	parser := NewParser()
	entries, err := parser.ParseEntries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bookmarks with no text should be skipped
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (bookmark skipped), got %d", len(entries))
	}
}

func TestParser_ParseEntries_LocationOnlyFormat(t *testing.T) {
	input := `Fahrenheit 451 (Ray Bradbury)
- Your Highlight at location 784-785 | Added on Saturday, 26 March 2016 18:37:26

Who knows who might be the target of the well-read man?
==========
`

	parser := NewParser()
	entries, err := parser.ParseEntries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Location != 784 {
		t.Errorf("expected location 784, got %d", entry.Location)
	}
	if entry.LocationEnd != 785 {
		t.Errorf("expected location end 785, got %d", entry.LocationEnd)
	}
	if entry.Page != 0 {
		t.Errorf("expected page 0, got %d", entry.Page)
	}
}

func TestParser_ParseEntries_NoAuthor(t *testing.T) {
	input := `Harry_Potter_und_die_Kammer_des_Schreckens
- Your Highlight on page 207-207 | Added on Monday, April 21, 2025 8:55:24 PM

Harry drehte sich auf die Seite
==========
`

	parser := NewParser()
	entries, err := parser.ParseEntries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "Harry_Potter_und_die_Kammer_des_Schreckens" {
		t.Errorf("expected title without author, got '%s'", entry.Title)
	}
	if entry.Author != "" {
		t.Errorf("expected empty author, got '%s'", entry.Author)
	}
	if entry.Page != 207 {
		t.Errorf("expected page 207, got %d", entry.Page)
	}
	if entry.PageEnd != 207 {
		t.Errorf("expected page end 207, got %d", entry.PageEnd)
	}
}

func TestParser_ParseEntries_MultiLineHighlight(t *testing.T) {
	input := `Test Book (Test Author)
- Your Highlight on page 1 | Location 10-15 | Added on Wednesday, January 1, 2025 12:00:00 PM

This highlight spans
multiple lines of text
that should be preserved.
==========
`

	parser := NewParser()
	entries, err := parser.ParseEntries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	expectedText := "This highlight spans\nmultiple lines of text\nthat should be preserved."
	if entries[0].Text != expectedText {
		t.Errorf("expected multiline text '%s', got '%s'", expectedText, entries[0].Text)
	}
}

func TestParser_ParseEntries_EmptyHighlight(t *testing.T) {
	input := `Test Book (Test Author)
- Your Highlight on Location 275 | Added on Monday, January 6, 2025 3:10:00 PM


==========
`

	parser := NewParser()
	entries, err := parser.ParseEntries(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty highlights should be skipped
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (empty content skipped), got %d", len(entries))
	}
}

func TestParser_Parse_GroupsIntoBooks(t *testing.T) {
	f, err := os.Open("testdata/sample_clippings.txt")
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	parser := NewParser()
	books, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 books: The_Power_of_Now, Harry_Potter, The Selfish Gene, Fahrenheit 451
	if len(books) != 4 {
		t.Fatalf("expected 4 books, got %d", len(books))
	}

	// Check Harry Potter book has 2 highlights
	var harryPotter *entities.Book
	for i := range books {
		if strings.Contains(books[i].Title, "Harry_Potter") {
			harryPotter = &books[i]
			break
		}
	}
	if harryPotter == nil {
		t.Fatal("Harry Potter book not found")
	}
	if len(harryPotter.Highlights) != 2 {
		t.Errorf("expected 2 highlights for Harry Potter, got %d", len(harryPotter.Highlights))
	}

	// Check all books have kindle source
	for _, book := range books {
		if book.Source.Name != "kindle" {
			t.Errorf("expected source 'kindle', got '%s'", book.Source.Name)
		}
	}
}

func TestParser_Parse_AttachesNotesToHighlights(t *testing.T) {
	f, err := os.Open("testdata/with_notes.txt")
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	parser := NewParser()
	books, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}

	book := books[0]

	// Should have 3 highlights: 2 regular + 1 note-only
	if len(book.Highlights) != 3 {
		t.Fatalf("expected 3 highlights, got %d", len(book.Highlights))
	}

	// First highlight should have attached note
	firstHighlight := book.Highlights[0]
	if firstHighlight.Note == "" {
		t.Error("expected first highlight to have attached note")
	}
	if !strings.Contains(firstHighlight.Note, "This is the key insight") {
		t.Errorf("expected note content, got '%s'", firstHighlight.Note)
	}

	// Find standalone note
	var standaloneNote *entities.Highlight
	for i := range book.Highlights {
		if book.Highlights[i].Style == entities.HighlightStyleNoteOnly {
			standaloneNote = &book.Highlights[i]
			break
		}
	}
	if standaloneNote == nil {
		t.Fatal("standalone note not found")
	}
	if !strings.Contains(standaloneNote.Note, "Standalone note") {
		t.Errorf("unexpected standalone note content: %s", standaloneNote.Note)
	}
}

func TestParser_Parse_EdgeCases(t *testing.T) {
	f, err := os.Open("testdata/edge_cases.txt")
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer f.Close()

	parser := NewParser()
	books, err := parser.Parse(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 books: Greenlights (1 highlight, empty one skipped),
	// Multi-Line Book (1 highlight), Book Without Author (1 highlight)
	// Empty_Content_Book should be skipped entirely
	if len(books) != 3 {
		t.Fatalf("expected 3 books, got %d", len(books))
	}

	// Verify book without author
	var noAuthorBook *entities.Book
	for i := range books {
		if books[i].Title == "Book Without Author" {
			noAuthorBook = &books[i]
			break
		}
	}
	if noAuthorBook == nil {
		t.Fatal("Book Without Author not found")
	}
	if noAuthorBook.Author != "" {
		t.Errorf("expected empty author, got '%s'", noAuthorBook.Author)
	}

	// Verify multi-line book has special characters in title
	var multiLineBook *entities.Book
	for i := range books {
		if strings.Contains(books[i].Title, "Multi-Line") {
			multiLineBook = &books[i]
			break
		}
	}
	if multiLineBook == nil {
		t.Fatal("Multi-Line book not found")
	}
	if multiLineBook.Author != "Jane Doe-Smith" {
		t.Errorf("expected author 'Jane Doe-Smith', got '%s'", multiLineBook.Author)
	}
}

func TestParseTitleAuthor(t *testing.T) {
	tests := []struct {
		input          string
		expectedTitle  string
		expectedAuthor string
	}{
		{
			input:          "The_Power_of_Now (Eckhart Tolle)",
			expectedTitle:  "The_Power_of_Now",
			expectedAuthor: "Eckhart Tolle",
		},
		{
			input:          "The Selfish Gene: 30th Anniversary Edition (Richard Dawkins)",
			expectedTitle:  "The Selfish Gene: 30th Anniversary Edition",
			expectedAuthor: "Richard Dawkins",
		},
		{
			input:          "Harry_Potter_und_die_Kammer_des_Schreckens",
			expectedTitle:  "Harry_Potter_und_die_Kammer_des_Schreckens",
			expectedAuthor: "",
		},
		{
			input:          "Book With (Nested (Parentheses)) (Author Name)",
			expectedTitle:  "Book With (Nested (Parentheses))",
			expectedAuthor: "Author Name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			title, author := parseTitleAuthor(tt.input)
			if title != tt.expectedTitle {
				t.Errorf("expected title '%s', got '%s'", tt.expectedTitle, title)
			}
			if author != tt.expectedAuthor {
				t.Errorf("expected author '%s', got '%s'", tt.expectedAuthor, author)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
	}{
		{
			input:    "- Your Highlight on page 8 | Location 64-64 | Added on Tuesday, April 15, 2025 10:16:21 PM",
			expected: time.Date(2025, 4, 15, 22, 16, 21, 0, time.UTC),
		},
		{
			input:    "- Your Highlight on page 92 | location 1406-1407 | Added on Saturday, 26 March 2016 14:59:39",
			expected: time.Date(2016, 3, 26, 14, 59, 39, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseDate(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParsePageRange(t *testing.T) {
	tests := []struct {
		input        string
		expectedPage int
		expectedEnd  int
	}{
		{"on page 8", 8, 0},
		{"on page 207-207", 207, 207},
		{"page 1-5", 1, 5},
		{"no page here", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			page, end := parsePageRange(tt.input)
			if page != tt.expectedPage {
				t.Errorf("expected page %d, got %d", tt.expectedPage, page)
			}
			if end != tt.expectedEnd {
				t.Errorf("expected end %d, got %d", tt.expectedEnd, end)
			}
		})
	}
}

func TestParseLocationRange(t *testing.T) {
	tests := []struct {
		input       string
		expectedLoc int
		expectedEnd int
	}{
		{"Location 64-64", 64, 64},
		{"location 1406-1407", 1406, 1407},
		{"at location 784-785", 784, 785},
		{"Location 307", 307, 0},
		{"no location here", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			loc, end := parseLocationRange(tt.input)
			if loc != tt.expectedLoc {
				t.Errorf("expected location %d, got %d", tt.expectedLoc, loc)
			}
			if end != tt.expectedEnd {
				t.Errorf("expected end %d, got %d", tt.expectedEnd, end)
			}
		})
	}
}
