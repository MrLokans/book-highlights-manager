package importers

import (
	"fmt"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/utils"
)

// MoonReaderHighlight represents a single highlight from Moon+ Reader.
type MoonReaderHighlight struct {
	ID             int64  `json:"id"`
	BookTitle      string `json:"book_title"`
	Filename       string `json:"filename"`
	HighlightColor string `json:"highlight_color"`
	TimeMs         int64  `json:"time"`
	Bookmark       string `json:"bookmark"`
	Note           string `json:"note"`
	Original       string `json:"original"`
	Underline      int    `json:"underline"`
	Strikethrough  int    `json:"strikethrough"`
}

// MoonReaderConverter converts Moon+ Reader highlights to the common format.
type MoonReaderConverter struct {
	Highlights []MoonReaderHighlight
}

// NewMoonReaderConverter creates a converter for Moon+ Reader highlights.
func NewMoonReaderConverter(highlights []MoonReaderHighlight) *MoonReaderConverter {
	return &MoonReaderConverter{Highlights: highlights}
}

// Convert implements Converter interface.
func (c *MoonReaderConverter) Convert() ([]RawHighlight, Source) {
	highlights := make([]RawHighlight, 0, len(c.Highlights))

	for _, h := range c.Highlights {
		author := utils.ExtractAuthorFromFilename(h.Filename, h.BookTitle)

		// Determine highlight style
		style := entities.HighlightStyleHighlight
		if h.Underline != 0 {
			style = entities.HighlightStyleUnderline
		} else if h.Strikethrough != 0 {
			style = entities.HighlightStyleStrikethrough
		}

		// Get text (prefer original over note)
		text := h.Original
		noteText := h.Note
		if text == "" {
			text = h.Note
			noteText = ""
		}

		// Convert color
		color, _ := utils.InternalColorToHexARGB(h.HighlightColor)

		highlights = append(highlights, RawHighlight{
			BookTitle:     h.BookTitle,
			BookAuthor:    author,
			Text:          text,
			Note:          noteText,
			Color:         color,
			Style:         style,
			HighlightedAt: time.UnixMilli(h.TimeMs).Format(time.RFC3339),
			Chapter:       h.Bookmark,
			ExternalID:    fmt.Sprintf("%d", h.ID),
			FilePath:      h.Filename,
		})
	}

	return highlights, Source{Name: "moonreader"}
}

// Compile-time interface check
var _ Converter = (*MoonReaderConverter)(nil)
