package exporters

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/utils"
)

type MarkdownExporter struct {
	ExportDir     string // Directory for markdown exports
	IndexFileName string
	currentBook   entities.Book
	Result        ExportResult
}

func NewMarkdownExporter(exportDir string) *MarkdownExporter {
	return &MarkdownExporter{
		ExportDir:     exportDir,
		IndexFileName: "index.md",
		Result:        ExportResult{},
		currentBook:   entities.Book{},
	}
}

// ErrExportDirNotConfigured is returned when the export directory is not configured
var ErrExportDirNotConfigured = fmt.Errorf("obsidian export directory not configured")

func (exporter *MarkdownExporter) ensureDirs() (string, error) {
	// Check if export directory is configured
	if exporter.ExportDir == "" {
		return "", ErrExportDirNotConfigured
	}

	if _, err := os.Stat(exporter.ExportDir); os.IsNotExist(err) {
		return "", fmt.Errorf("export directory does not exist: %s", exporter.ExportDir)
	}

	return exporter.ExportDir, nil
}

func (exporter *MarkdownExporter) exportBook(book entities.Book, exportDir string) (string, error) {
	// Determine source folder
	sourceFolder := "unknown"
	if book.Source.Name != "" {
		sourceFolder = book.Source.Name
	}

	// Create source subdirectory
	sourceDir := fmt.Sprintf("%s/%s", exportDir, sourceFolder)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create source directory: %w", err)
	}

	// Sanitize title for filename
	safeTitle := sanitizeFilename(book.Title)
	outputPath := fmt.Sprintf("%s/%s.md", sourceDir, safeTitle)

	fmt.Printf("Exporting book: %s to %s\n", book.Title, outputPath)

	outpotBookFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outpotBookFile.Close()

	// Use the shared markdown generation function
	content := GenerateMarkdown(&book)
	exporter.Result.HighlightsProcessed += len(book.Highlights)

	_, writeError := outpotBookFile.WriteString(content)
	if writeError != nil {
		return "", writeError
	}
	return outputPath, nil
}

// sanitizeFilename removes/replaces characters that are invalid in filenames
func sanitizeFilename(name string) string {
	// Replace problematic characters with safe alternatives
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "'",
		"<", "",
		">", "",
		"|", "-",
	)
	return replacer.Replace(name)
}

func GenerateMarkdown(book *entities.Book) string {
	var builder strings.Builder

	sourceFolder := "unknown"
	if book.Source.Name != "" {
		sourceFolder = book.Source.Name
	}

	currentDateTime := time.Now().Format("2006-01-02")
	fmt.Fprintf(&builder, "---\n")
	fmt.Fprintf(&builder, "content_source: %s\n", sourceFolder)
	fmt.Fprintf(&builder, "content_type: book_highlights\n")
	fmt.Fprintf(&builder, "created_at: %s\n", currentDateTime)
	fmt.Fprintf(&builder, "title: \"%s\"\n", strings.ReplaceAll(book.Title, "\"", "\\\""))
	fmt.Fprintf(&builder, "author: \"%s\"\n", strings.ReplaceAll(book.Author, "\"", "\\\""))
	fmt.Fprintf(&builder, "highlights_count: %d\n", len(book.Highlights))

	// Include book tags in YAML frontmatter
	tags := collectAllTags(book)
	if len(tags) > 0 {
		fmt.Fprintf(&builder, "tags: [%s]\n", strings.Join(tags, ", "))
	} else {
		fmt.Fprintf(&builder, "tags: [highlights, books]\n")
	}

	// Count favorites for summary
	favoriteCount := countFavorites(book.Highlights)
	if favoriteCount > 0 {
		fmt.Fprintf(&builder, "favorite_count: %d\n", favoriteCount)
	}

	fmt.Fprintf(&builder, "---\n\n")

	// Book header with author
	fmt.Fprintf(&builder, "# %s\n", book.Title)
	if book.Author != "" {
		fmt.Fprintf(&builder, "*by %s*\n\n", book.Author)
	} else {
		fmt.Fprintf(&builder, "\n")
	}

	fmt.Fprintf(&builder, "## Highlights\n\n")

	for _, highlight := range book.Highlights {
		renderHighlight(&builder, &highlight)
	}

	return builder.String()
}

// renderHighlight renders a single highlight using Obsidian callout syntax
func renderHighlight(builder *strings.Builder, highlight *entities.Highlight) {
	calloutType := getCalloutType(highlight)

	// Build callout header with timestamp
	timestamp := formatHighlightTime(highlight)

	// Add chapter/bookmark info if available
	locationInfo := ""
	if highlight.Chapter != "" {
		locationInfo = fmt.Sprintf(" • %s", highlight.Chapter)
	}

	// Add favorite marker to callout header
	favoriteMarker := ""
	if highlight.IsFavorite {
		favoriteMarker = "⭐ "
	}

	fmt.Fprintf(builder, "> [!%s] %s%s%s\n", calloutType, favoriteMarker, timestamp, locationInfo)

	// Format the highlight text with proper callout indentation
	text := strings.TrimSpace(highlight.Text)
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintf(builder, "> %s\n", line)
	}

	// Add note if present
	if highlight.Note != "" {
		fmt.Fprintf(builder, "> \n")
		fmt.Fprintf(builder, "> **Note:** %s\n", highlight.Note)
	}

	// Add style indicators for underline/strikethrough
	var indicators []string
	if highlight.Style == entities.HighlightStyleUnderline {
		indicators = append(indicators, "📝 underlined")
	}
	if highlight.Style == entities.HighlightStyleStrikethrough {
		indicators = append(indicators, "❌ crossed out")
	}
	if len(indicators) > 0 {
		fmt.Fprintf(builder, "> \n")
		fmt.Fprintf(builder, "> *%s*\n", strings.Join(indicators, " • "))
	}

	// Add highlight-specific tags if present
	if len(highlight.Tags) > 0 {
		highlightTags := make([]string, len(highlight.Tags))
		for i, tag := range highlight.Tags {
			highlightTags[i] = "#" + strings.ReplaceAll(tag.Name, " ", "-")
		}
		fmt.Fprintf(builder, "> \n")
		fmt.Fprintf(builder, "> Tags: %s\n", strings.Join(highlightTags, " "))
	}

	fmt.Fprintf(builder, "\n")
}

// getCalloutType determines the Obsidian callout type based on highlight properties
func getCalloutType(highlight *entities.Highlight) string {
	// Style takes priority
	switch highlight.Style {
	case entities.HighlightStyleStrikethrough:
		return "failure"
	case entities.HighlightStyleUnderline:
		return "success"
	}

	// Then check color
	if highlight.Color != "" {
		return utils.ColorToCalloutType(highlight.Color)
	}

	return "quote"
}

// formatHighlightTime returns a formatted timestamp string for the highlight
func formatHighlightTime(highlight *entities.Highlight) string {
	if !highlight.HighlightedAt.IsZero() {
		return highlight.HighlightedAt.Format("2006-01-02 15:04")
	}
	if highlight.Time != "" { //nolint:staticcheck // Using deprecated field for backward compatibility
		return highlight.Time //nolint:staticcheck // Using deprecated field for backward compatibility
	}
	return "(no date)"
}

// collectAllTags gathers unique tags from book and all its highlights
func collectAllTags(book *entities.Book) []string {
	tagMap := make(map[string]bool)

	// Always include base tags
	tagMap["highlights"] = true
	tagMap["books"] = true

	// Add book-level tags
	for _, tag := range book.Tags {
		tagMap[tag.Name] = true
	}

	// Add highlight-level tags
	for _, highlight := range book.Highlights {
		for _, tag := range highlight.Tags {
			tagMap[tag.Name] = true
		}
	}

	// Convert to slice
	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	return tags
}

// countFavorites counts how many highlights are marked as favorites
func countFavorites(highlights []entities.Highlight) int {
	count := 0
	for _, h := range highlights {
		if h.IsFavorite {
			count++
		}
	}
	return count
}

// GenerateVocabularyMarkdown generates markdown content for all vocabulary words
func GenerateVocabularyMarkdown(words []entities.Word) string {
	var builder strings.Builder

	currentDateTime := time.Now().Format("2006-01-02")
	fmt.Fprintf(&builder, "---\n")
	fmt.Fprintf(&builder, "content_type: vocabulary\n")
	fmt.Fprintf(&builder, "created_at: %s\n", currentDateTime)
	fmt.Fprintf(&builder, "word_count: %d\n", len(words))

	// Count enriched words
	enrichedCount := 0
	for _, w := range words {
		if w.Status == entities.WordStatusEnriched {
			enrichedCount++
		}
	}
	fmt.Fprintf(&builder, "enriched_count: %d\n", enrichedCount)
	fmt.Fprintf(&builder, "tags: [vocabulary, words]\n")
	fmt.Fprintf(&builder, "---\n\n")
	fmt.Fprintf(&builder, "# Vocabulary\n\n")
	fmt.Fprintf(&builder, "A collection of %d words saved from reading highlights.\n\n", len(words))

	for _, word := range words {
		fmt.Fprintf(&builder, "## %s\n\n", word.Word)

		// Add source info if available
		if word.SourceBookTitle != "" {
			fmt.Fprintf(&builder, "**Source:** %s", word.SourceBookTitle)
			if word.SourceBookAuthor != "" {
				fmt.Fprintf(&builder, " by %s", word.SourceBookAuthor)
			}
			fmt.Fprintf(&builder, "\n\n")
		}

		// Add context if available
		if word.Context != "" {
			fmt.Fprintf(&builder, "> %s\n\n", strings.ReplaceAll(word.Context, "\n", "\n> "))
		}

		// Add definitions if available
		if len(word.Definitions) > 0 {
			fmt.Fprintf(&builder, "### Definitions\n\n")
			for _, def := range word.Definitions {
				if def.PartOfSpeech != "" {
					fmt.Fprintf(&builder, "**%s**\n", def.PartOfSpeech)
				}
				fmt.Fprintf(&builder, "- %s\n", def.Definition)
				if def.Example != "" {
					fmt.Fprintf(&builder, "  - *Example: %s*\n", def.Example)
				}
			}
			fmt.Fprintf(&builder, "\n")
		}

		fmt.Fprintf(&builder, "---\n\n")
	}

	return builder.String()
}

// ExportVocabulary exports all vocabulary words to a single markdown file
func (exporter *MarkdownExporter) ExportVocabulary(words []entities.Word) error {
	// Check if export directory is configured
	if exporter.ExportDir == "" {
		return ErrExportDirNotConfigured
	}

	exportDir, err := exporter.ensureDirs()
	if err != nil {
		return err
	}

	outputPath := fmt.Sprintf("%s/vocabulary.md", exportDir)
	fmt.Printf("Exporting vocabulary (%d words) to %s\n", len(words), outputPath)

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create vocabulary file: %w", err)
	}
	defer file.Close()

	content := GenerateVocabularyMarkdown(words)
	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write vocabulary file: %w", err)
	}

	return nil
}

func (exporter *MarkdownExporter) Export(books []entities.Book) (ExportResult, error) {
	// Reset result state for each export
	exporter.Result = ExportResult{}

	exportDir, dirsErr := exporter.ensureDirs()
	if dirsErr != nil {
		return ExportResult{}, dirsErr
	}

	for _, book := range books {
		exporter.currentBook = book
		_, err := exporter.exportBook(book, exportDir)
		// TODO: log error instead and continue
		if err != nil {
			return ExportResult{}, err
		}
		exporter.Result.BooksProcessed++
	}

	return exporter.Result, nil
}
