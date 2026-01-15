package importers

// ReadwiseHighlight represents a single highlight from the Readwise API.
type ReadwiseHighlight struct {
	Text          string `json:"text"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	SourceType    string `json:"source_type"`
	Category      string `json:"category"`
	Note          string `json:"note"`
	Page          int    `json:"location"`
	LocationType  string `json:"location_type"`
	HighlightedAt string `json:"highlighted_at"`
	ID            string `json:"id"`
}

// ReadwiseConverter converts Readwise API highlights to the common format.
type ReadwiseConverter struct {
	Highlights []ReadwiseHighlight
}

// NewReadwiseConverter creates a converter for Readwise API highlights.
func NewReadwiseConverter(highlights []ReadwiseHighlight) *ReadwiseConverter {
	return &ReadwiseConverter{Highlights: highlights}
}

// Convert implements Converter interface.
func (c *ReadwiseConverter) Convert() ([]RawHighlight, Source) {
	highlights := make([]RawHighlight, 0, len(c.Highlights))

	for _, h := range c.Highlights {
		highlights = append(highlights, RawHighlight{
			BookTitle:     h.Title,
			BookAuthor:    h.Author,
			Text:          h.Text,
			Note:          h.Note,
			Page:          h.Page,
			HighlightedAt: h.HighlightedAt,
			ExternalID:    h.ID,
		})
	}

	return highlights, Source{Name: "readwise"}
}

// Compile-time interface check
var _ Converter = (*ReadwiseConverter)(nil)
