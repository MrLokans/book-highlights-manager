package moonreader

import (
	"time"

	"github.com/mrlokans/assistant/internal/utils"
)

// MoonReaderNote represents a note/highlight from MoonReader's backup database.
// This is the raw format as stored in the MoonReader SQLite database.
type MoonReaderNote struct {
	ID             int64  // _id from MoonReader
	BookTitle      string // book
	Filename       string // filename
	HighlightColor string // highlightColor (stored as signed int string)
	TimeMs         int64  // time in milliseconds
	Bookmark       string // bookmark
	Note           string // note (user annotation)
	Original       string // original (highlighted text)
	Underline      int    // underline flag
	Strikethrough  int    // strikethrough flag
}

// GetTime converts the millisecond timestamp to a time.Time
func (n *MoonReaderNote) GetTime() time.Time {
	return time.UnixMilli(n.TimeMs)
}

// GetColorHex returns the highlight color as ARGB hex string
func (n *MoonReaderNote) GetColorHex() string {
	hex, err := utils.InternalColorToHexARGB(n.HighlightColor)
	if err != nil {
		return "#FFFFFF00" // Default to yellow
	}
	return hex
}

// IsUnderlined returns true if the highlight is underlined
func (n *MoonReaderNote) IsUnderlined() bool {
	return n.Underline != 0
}

// IsStrikethrough returns true if the highlight has strikethrough
func (n *MoonReaderNote) IsStrikethrough() bool {
	return n.Strikethrough != 0
}

// GetText returns the highlight text, preferring original over note
func (n *MoonReaderNote) GetText() string {
	if n.Original != "" {
		return n.Original
	}
	return n.Note
}

// GetAuthor attempts to extract author from the filename
func (n *MoonReaderNote) GetAuthor() string {
	return utils.ExtractAuthorFromFilename(n.Filename, n.BookTitle)
}

// LocalNote represents a note stored in our local database.
// This is similar to MoonReaderNote but with processed fields.
type LocalNote struct {
	ExternalID    string    // exported_id (ID from MoonReader as string)
	BookTitle     string    // book_title
	Filename      string    // filename
	Color         string    // color (stored as original string for roundtrip)
	Time          time.Time // time as parsed datetime
	TimeMs        int64     // original time in milliseconds
	Bookmark      string    // bookmark
	NoteText      string    // note (user annotation)
	Original      string    // original (highlighted text)
	Underline     bool      // underline flag
	Strikethrough bool      // strikethrough flag
}

// GetText returns the highlight text, preferring original over note
func (n *LocalNote) GetText() string {
	if n.Original != "" {
		return n.Original
	}
	return n.NoteText
}

// GetColorHex returns the highlight color as ARGB hex string
func (n *LocalNote) GetColorHex() string {
	hex, err := utils.InternalColorToHexARGB(n.Color)
	if err != nil {
		return "#FFFFFF00" // Default to yellow
	}
	return hex
}

// GetAuthor attempts to extract author from the filename
func (n *LocalNote) GetAuthor() string {
	return utils.ExtractAuthorFromFilename(n.Filename, n.BookTitle)
}
