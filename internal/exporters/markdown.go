package exporters

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

type MarkdownExporter struct {
	ObsidianVaultDir    string
	ObisidianExportPath string
	IndexFileName       string
	currentBook         entities.Book
	Result              ExportResult
}

func NewMarkdownExporter(vaultDir string, exportPath string) *MarkdownExporter {
	return &MarkdownExporter{
		ObsidianVaultDir:    vaultDir,
		ObisidianExportPath: exportPath,
		IndexFileName:       "index.md",
		Result:              ExportResult{},
		currentBook:         entities.Book{},
	}
}

func (exporter *MarkdownExporter) ensureDirs() (string, error) {
	if _, err := os.Stat(exporter.ObsidianVaultDir); os.IsNotExist(err) {
		return "", err
	}

	// Create export dir within the vault if does not yet exist
	exportDir := fmt.Sprintf("%s/%s", exporter.ObsidianVaultDir, exporter.ObisidianExportPath)
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		if err := os.Mkdir(exportDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create export directory: %w", err)
		}
	}
	return exportDir, nil
}

func (exporter *MarkdownExporter) exportBook(book entities.Book, exportDir string) (string, error) {
	// Check that obisidan vault dir exists and accessible, fail otherwise

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

	outputPath := fmt.Sprintf("%s/%s.md", sourceDir, book.Title)

	fmt.Printf("Exporting book: %s\n to %s", book.Title, outputPath)

	outpotBookFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outpotBookFile.Close()
	var bookContentBuilder strings.Builder

	fmt.Fprintf(&bookContentBuilder, "---\n")
	currentDateTime := time.Now().Format("2006-01-02")
	fmt.Fprintf(&bookContentBuilder, "content_source: %s\n", sourceFolder)
	fmt.Fprintf(&bookContentBuilder, "content_type: book_highlights\n")
	fmt.Fprintf(&bookContentBuilder, "created_at: %s\n", currentDateTime)
	fmt.Fprintf(&bookContentBuilder, "title: %s\n", book.Title)
	fmt.Fprintf(&bookContentBuilder, "author: %s\n", book.Author)
	fmt.Fprintf(&bookContentBuilder, "tags: highlights, books\n")
	fmt.Fprintf(&bookContentBuilder, "---\n")
	fmt.Fprintf(&bookContentBuilder, "## Highlights:\n")

	for _, highlight := range book.Highlights {
		fmt.Fprintf(&bookContentBuilder, "### (taken_at: %s)\n", highlight.Time) //nolint:staticcheck // Using deprecated field for backward compatibility
		fmt.Fprintf(&bookContentBuilder, "%s\n\n", highlight.Text)
		exporter.Result.HighlightsProcessed++
	}

	_, indexWriteError := outpotBookFile.WriteString(bookContentBuilder.String())
	if indexWriteError != nil {
		return "", indexWriteError
	}
	return outputPath, nil
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
	fmt.Fprintf(&builder, "tags: highlights, books\n")
	fmt.Fprintf(&builder, "---\n\n")
	fmt.Fprintf(&builder, "## Highlights\n\n")

	for _, highlight := range book.Highlights {
		if !highlight.HighlightedAt.IsZero() {
			fmt.Fprintf(&builder, "### %s\n\n", highlight.HighlightedAt.Format("2006-01-02 15:04"))
		} else if highlight.Time != "" { //nolint:staticcheck // Using deprecated field for backward compatibility
			fmt.Fprintf(&builder, "### %s\n\n", highlight.Time) //nolint:staticcheck // Using deprecated field for backward compatibility
		}
		fmt.Fprintf(&builder, "> %s\n\n", strings.ReplaceAll(highlight.Text, "\n", "\n> "))
		if highlight.Note != "" {
			fmt.Fprintf(&builder, "**Note:** %s\n\n", highlight.Note)
		}
	}

	return builder.String()
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
