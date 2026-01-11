package moonreader

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/utils"
)

// ReadingStats contains statistics about highlights for a book
type ReadingStats struct {
	ReadingSpanDays      int
	FirstHighlight       time.Time
	LastHighlight        time.Time
	UnderlinedCount      int
	StrikethroughCount   int
	ColorDistribution    map[string]int
	AvgHighlightsPerDay  float64
	TotalHighlights      int
}

// ObsidianRenderer renders notes to Obsidian-compatible markdown
type ObsidianRenderer struct{}

// NewObsidianRenderer creates a new ObsidianRenderer
func NewObsidianRenderer() *ObsidianRenderer {
	return &ObsidianRenderer{}
}

// GetCalloutType determines the callout type based on highlight color and formatting
func (r *ObsidianRenderer) GetCalloutType(note *LocalNote) string {
	// Check formatting first
	if note.Strikethrough {
		return "failure"
	}
	if note.Underline {
		return "success"
	}

	// Map colors to callout types
	hexColor := note.GetColorHex()
	return utils.ColorToCalloutType(hexColor)
}

// FormatRelativeTime formats a time as relative (e.g., "2 days ago") or absolute if old
func (r *ObsidianRenderer) FormatRelativeTime(noteTime time.Time) string {
	now := time.Now()
	diff := now.Sub(noteTime)

	days := int(diff.Hours() / 24)
	hours := int(diff.Hours())
	minutes := int(diff.Minutes())

	switch {
	case days == 0:
		if hours < 1 {
			if minutes <= 1 {
				return "just now"
			}
			return fmt.Sprintf("%d minutes ago", minutes)
		}
		return fmt.Sprintf("%d hours ago", hours)
	case days == 1:
		return "yesterday"
	case days < 7:
		return fmt.Sprintf("%d days ago", days)
	case days < 30:
		weeks := days / 7
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		return noteTime.Format("January 02, 2006")
	}
}

// RenderNote renders a single note as an Obsidian callout
func (r *ObsidianRenderer) RenderNote(note *LocalNote) string {
	var sb strings.Builder

	calloutType := r.GetCalloutType(note)
	relativeTime := r.FormatRelativeTime(note.Time)

	// Add bookmark context if available
	bookmarkInfo := ""
	if strings.TrimSpace(note.Bookmark) != "" {
		bookmarkInfo = fmt.Sprintf(" • %s", note.Bookmark)
	}

	sb.WriteString(fmt.Sprintf("> [!%s] %s%s\n", calloutType, relativeTime, bookmarkInfo))

	// Format the note text with proper indentation for callouts
	noteText := strings.TrimSpace(note.GetText())
	for _, line := range strings.Split(noteText, "\n") {
		sb.WriteString(fmt.Sprintf("> %s\n", line))
	}

	// Add formatting indicators
	var indicators []string
	if note.Underline {
		indicators = append(indicators, "📝 underlined")
	}
	if note.Strikethrough {
		indicators = append(indicators, "❌ crossed out")
	}

	if len(indicators) > 0 {
		sb.WriteString("> \n")
		sb.WriteString(fmt.Sprintf("> *%s*\n", strings.Join(indicators, " • ")))
	}

	sb.WriteString("\n")
	return sb.String()
}

// CalculateReadingStats calculates statistics about highlights
func (r *ObsidianRenderer) CalculateReadingStats(notes []*LocalNote) *ReadingStats {
	if len(notes) == 0 {
		return nil
	}

	stats := &ReadingStats{
		ColorDistribution: make(map[string]int),
		TotalHighlights:   len(notes),
	}

	var firstTime, lastTime time.Time
	for i, note := range notes {
		if i == 0 || note.Time.Before(firstTime) {
			firstTime = note.Time
		}
		if i == 0 || note.Time.After(lastTime) {
			lastTime = note.Time
		}

		if note.Underline {
			stats.UnderlinedCount++
		}
		if note.Strikethrough {
			stats.StrikethroughCount++
		}

		colorHex := note.GetColorHex()
		stats.ColorDistribution[colorHex]++
	}

	stats.FirstHighlight = firstTime
	stats.LastHighlight = lastTime
	stats.ReadingSpanDays = int(lastTime.Sub(firstTime).Hours() / 24)

	if stats.ReadingSpanDays > 0 {
		stats.AvgHighlightsPerDay = float64(len(notes)) / float64(stats.ReadingSpanDays)
	} else {
		stats.AvgHighlightsPerDay = float64(len(notes))
	}

	return stats
}

// GroupNotesByTimeframe groups notes by reading sessions (same day)
func (r *ObsidianRenderer) GroupNotesByTimeframe(notes []*LocalNote) map[string][]*LocalNote {
	grouped := make(map[string][]*LocalNote)
	for _, note := range notes {
		dateKey := note.Time.Format("2006-01-02")
		grouped[dateKey] = append(grouped[dateKey], note)
	}
	return grouped
}

// RenderBook renders a complete book with all its notes
func (r *ObsidianRenderer) RenderBook(book *BookContainer) string {
	var sb strings.Builder

	stats := r.CalculateReadingStats(book.Notes)

	// Enhanced frontmatter
	sb.WriteString("---\n")
	if book.Author != "" {
		sb.WriteString(fmt.Sprintf("book_author: %s\n", book.Author))
	}
	sb.WriteString(fmt.Sprintf("book_title: %s\n", book.Title))
	sb.WriteString("content_source: moonreader_import\n")
	sb.WriteString("content_type: book_highlights\n")
	sb.WriteString(fmt.Sprintf("exported_at: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("highlights_count: %d\n", len(book.Notes)))

	if stats != nil {
		sb.WriteString(fmt.Sprintf("reading_span_days: %d\n", stats.ReadingSpanDays))
		sb.WriteString(fmt.Sprintf("first_highlight: %s\n", stats.FirstHighlight.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("last_highlight: %s\n", stats.LastHighlight.Format("2006-01-02")))
		if stats.UnderlinedCount > 0 {
			sb.WriteString(fmt.Sprintf("underlined_highlights: %d\n", stats.UnderlinedCount))
		}
		if stats.StrikethroughCount > 0 {
			sb.WriteString(fmt.Sprintf("crossed_out_highlights: %d\n", stats.StrikethroughCount))
		}
	}

	// Tags
	authorTag := "unknown-author"
	if book.Author != "" {
		authorTag = strings.ToLower(strings.ReplaceAll(book.Author, " ", "-"))
	}
	sb.WriteString(fmt.Sprintf("tags: [highlights, books, %s]\n", authorTag))
	sb.WriteString("---\n\n")

	// Book header
	sb.WriteString(fmt.Sprintf("# %s\n", book.Title))
	if book.Author != "" {
		sb.WriteString(fmt.Sprintf("*by %s*\n\n", book.Author))
	}

	// Reading summary
	if stats != nil && len(book.Notes) > 0 {
		sb.WriteString("> [!info] Reading Summary\n")
		sb.WriteString(fmt.Sprintf("> **%d highlights** collected over **%d days**\n", stats.TotalHighlights, stats.ReadingSpanDays))
		if stats.AvgHighlightsPerDay > 1 {
			sb.WriteString(fmt.Sprintf("> Average: %.1f highlights per day\n", stats.AvgHighlightsPerDay))
		}
		sb.WriteString("\n")
	}

	// Group notes by reading sessions
	groupedNotes := r.GroupNotesByTimeframe(book.Notes)

	// Get sorted date keys
	var dateKeys []string
	for k := range groupedNotes {
		dateKeys = append(dateKeys, k)
	}
	sort.Strings(dateKeys)

	for _, date := range dateKeys {
		dailyNotes := groupedNotes[date]

		// Only show date headers if multiple days
		if len(groupedNotes) > 1 {
			readableDate, _ := time.Parse("2006-01-02", date)
			sb.WriteString(fmt.Sprintf("## %s\n\n", readableDate.Format("January 02, 2006")))
		}

		for _, note := range dailyNotes {
			sb.WriteString(r.RenderNote(note))
		}
	}

	return sb.String()
}

// ObsidianExporter exports notes to Obsidian-compatible markdown files
type ObsidianExporter struct {
	OutputDir string
	Renderer  *ObsidianRenderer
	accessor  *LocalDBAccessor
}

// NewObsidianExporter creates a new ObsidianExporter
func NewObsidianExporter(outputDir string, accessor *LocalDBAccessor) *ObsidianExporter {
	return &ObsidianExporter{
		OutputDir: outputDir,
		Renderer:  NewObsidianRenderer(),
		accessor:  accessor,
	}
}

// ExportResult contains information about an export operation
type ExportResult struct {
	ExportedFiles map[string]string // book title -> file path
	Errors        []string
}

// Export exports all books to markdown files
func (e *ObsidianExporter) Export() (*ExportResult, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(e.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	notesByBook, err := e.accessor.GetNotesByBook()
	if err != nil {
		return nil, fmt.Errorf("failed to get notes: %w", err)
	}

	if len(notesByBook) == 0 {
		return &ExportResult{ExportedFiles: make(map[string]string)}, nil
	}

	result := &ExportResult{
		ExportedFiles: make(map[string]string),
	}

	// Create moonreader subdirectory
	moonreaderDir := filepath.Join(e.OutputDir, "moonreader")
	if err := os.MkdirAll(moonreaderDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create moonreader directory: %w", err)
	}

	for bookTitle, notes := range notesByBook {
		book := NewBookContainer(bookTitle, notes)
		markdown := e.Renderer.RenderBook(book)

		// Create sanitized filename
		safeFilename := utils.SanitizeFilename(bookTitle)
		outputFile := filepath.Join(moonreaderDir, safeFilename+".md")

		// Write the file (overwrites existing)
		if err := os.WriteFile(outputFile, []byte(markdown), 0644); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to export '%s': %v", bookTitle, err))
			continue
		}

		result.ExportedFiles[bookTitle] = outputFile
	}

	return result, nil
}

// ExportSingleBook exports a single book by title
func (e *ObsidianExporter) ExportSingleBook(bookTitle string) (string, error) {
	notes, err := e.accessor.GetNotes()
	if err != nil {
		return "", fmt.Errorf("failed to get notes: %w", err)
	}

	// Filter notes for this book
	var bookNotes []*LocalNote
	for _, note := range notes {
		if note.BookTitle == bookTitle {
			bookNotes = append(bookNotes, note)
		}
	}

	if len(bookNotes) == 0 {
		return "", fmt.Errorf("no notes found for book: %s", bookTitle)
	}

	// Create moonreader subdirectory
	moonreaderDir := filepath.Join(e.OutputDir, "moonreader")
	if err := os.MkdirAll(moonreaderDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create moonreader directory: %w", err)
	}

	book := NewBookContainer(bookTitle, bookNotes)
	markdown := e.Renderer.RenderBook(book)

	// Create sanitized filename
	safeFilename := utils.SanitizeFilename(bookTitle)
	outputFile := filepath.Join(moonreaderDir, safeFilename+".md")

	// Write the file (overwrites existing)
	if err := os.WriteFile(outputFile, []byte(markdown), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return outputFile, nil
}
