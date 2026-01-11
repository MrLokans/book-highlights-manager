package parsers

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mrlokans/assistant/internal/entities"
)

type MarkdownParser struct {
	ExportDir string
}

func NewMarkdownParser(exportDir string) *MarkdownParser {
	return &MarkdownParser{
		ExportDir: exportDir,
	}
}

// ParseResult contains the results of parsing markdown files
type ParseResult struct {
	BooksProcessed      int `json:"books_processed"`
	HighlightsProcessed int `json:"highlights_processed"`
	BooksFailed         int `json:"books_failed"`
	HighlightsFailed    int `json:"highlights_failed"`
}

// ParseAllMarkdownFiles reads all .md files in the export directory and converts them to entities.Book
func (parser *MarkdownParser) ParseAllMarkdownFiles() ([]entities.Book, ParseResult, error) {
	var books []entities.Book
	result := ParseResult{}

	// Read all .md files in the export directory
	files, err := filepath.Glob(filepath.Join(parser.ExportDir, "*.md"))
	if err != nil {
		return nil, result, fmt.Errorf("failed to list markdown files: %w", err)
	}

	for _, file := range files {
		book, err := parser.ParseMarkdownFile(file)
		if err != nil {
			result.BooksFailed++
			continue
		}
		books = append(books, *book)
		result.BooksProcessed++
		result.HighlightsProcessed += len(book.Highlights)
	}

	return books, result, nil
}

// ParseAllMarkdownFilesRecursive recursively walks through a directory and parses all .md files
func (parser *MarkdownParser) ParseAllMarkdownFilesRecursive(rootDir string) ([]entities.Book, ParseResult, error) {
	var books []entities.Book
	result := ParseResult{}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return nil // Continue walking despite errors
		}

		// Skip directories and non-markdown files
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		log.Printf("Processing file: %s", path)

		book, parseErr := parser.ParseMarkdownFile(path)
		if parseErr != nil {
			log.Printf("Failed to parse file %s: %v", path, parseErr)
			result.BooksFailed++
			return nil // Continue processing other files
		}

		books = append(books, *book)
		result.BooksProcessed++
		result.HighlightsProcessed += len(book.Highlights)

		log.Printf("Successfully parsed book '%s' by %s with %d highlights",
			book.Title, book.Author, len(book.Highlights))

		return nil
	})

	if err != nil {
		return books, result, fmt.Errorf("failed to walk directory %s: %w", rootDir, err)
	}

	return books, result, nil
}

// ParseMarkdownFile reads a single markdown file and converts it to entities.Book
func (parser *MarkdownParser) ParseMarkdownFile(filePath string) (*entities.Book, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	book := &entities.Book{
		Highlights: make([]entities.Highlight, 0),
	}

	// Parse frontmatter
	if err := parser.parseFrontmatter(scanner, book); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Parse highlights
	if err := parser.parseHighlights(scanner, book); err != nil {
		return nil, fmt.Errorf("failed to parse highlights: %w", err)
	}

	return book, nil
}

// parseFrontmatter extracts title and author from YAML frontmatter or markdown headers
func (parser *MarkdownParser) parseFrontmatter(scanner *bufio.Scanner, book *entities.Book) error {
	if !scanner.Scan() {
		return fmt.Errorf("empty file")
	}

	firstLine := scanner.Text()

	// Check if it's a markdown header format (starts with #)
	if strings.HasPrefix(firstLine, "# ") {
		return parser.parseMarkdownHeader(firstLine, scanner, book)
	}

	// Check if it's YAML frontmatter format (starts with ---)
	if firstLine == "---" {
		return parser.parseYAMLFrontmatter(scanner, book)
	}

	return fmt.Errorf("unsupported frontmatter format: expected YAML frontmatter (---) or markdown header (#)")
}

// parseYAMLFrontmatter handles the original YAML frontmatter format
func (parser *MarkdownParser) parseYAMLFrontmatter(scanner *bufio.Scanner, book *entities.Book) error {
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			break // End of frontmatter
		}

		// Parse YAML-like key: value pairs
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "title", "book_title":
					book.Title = value
				case "author", "book_author":
					book.Author = value
				}
			}
		}
	}

	if book.Title == "" || book.Author == "" {
		return fmt.Errorf("missing required fields: title=%s, author=%s", book.Title, book.Author)
	}

	return nil
}

// parseMarkdownHeader handles the markdown header format
func (parser *MarkdownParser) parseMarkdownHeader(titleLine string, scanner *bufio.Scanner, book *entities.Book) error {
	// Extract title from the first line (remove "# " prefix)
	book.Title = strings.TrimSpace(strings.TrimPrefix(titleLine, "# "))

	// Look for author in the next few lines
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and separators
		if line == "" || line == "---" {
			continue
		}

		// Look for author line (format: "## Author: Name")
		if strings.HasPrefix(line, "## Author: ") {
			book.Author = strings.TrimSpace(strings.TrimPrefix(line, "## Author: "))
			break
		}

		// If we hit highlights section, stop looking for author
		if strings.HasPrefix(line, "## Highlights") {
			break
		}
	}

	// Extract title and author from the title line if author wasn't found separately
	if book.Author == "" {
		// Try to parse title like "Title: Subtitle" by "Author"
		titleParts := strings.Split(book.Title, " by ")
		if len(titleParts) == 2 {
			book.Title = strings.TrimSpace(titleParts[0])
			book.Author = strings.TrimSpace(titleParts[1])
		} else {
			// Try to parse title with colon separator
			colonParts := strings.Split(book.Title, ":")
			if len(colonParts) >= 2 {
				book.Title = strings.TrimSpace(colonParts[0])
				// Author might be in a separate line, so we'll set a placeholder
				book.Author = "Unknown Author"
			}
		}
	}

	if book.Title == "" {
		return fmt.Errorf("missing title")
	}

	// If we still don't have an author, set a default
	if book.Author == "" {
		book.Author = "Unknown Author"
	}

	return nil
}

// parseHighlights extracts highlights from the markdown content
func (parser *MarkdownParser) parseHighlights(scanner *bufio.Scanner, book *entities.Book) error {
	// Regex patterns for different highlight formats
	// Format 1: ### (taken_at: 2025-02-13T07:34:47+01:00)
	takenAtPattern := regexp.MustCompile(`^### \(taken_at: (.+)\)$`)
	// Format 2: ### 2022-10-02 08:07:58.549075
	timestampPattern := regexp.MustCompile(`^### (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?)$`)
	// Format 3: ### (Page: 0)
	pagePattern := regexp.MustCompile(`^### \(Page: (\d+)\)$`)

	var currentHighlight *entities.Highlight
	var highlightText strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is a new highlight header
		if matches := takenAtPattern.FindStringSubmatch(line); matches != nil {
			// Save previous highlight if exists
			if currentHighlight != nil {
				currentHighlight.Text = strings.TrimSpace(highlightText.String())
				book.Highlights = append(book.Highlights, *currentHighlight)
			}

			// Start new highlight
			currentHighlight = &entities.Highlight{
				Time: matches[1],
				Page: 0, // Page info not available in this format
			}
			highlightText.Reset()
		} else if matches := timestampPattern.FindStringSubmatch(line); matches != nil {
			// Save previous highlight if exists
			if currentHighlight != nil {
				currentHighlight.Text = strings.TrimSpace(highlightText.String())
				book.Highlights = append(book.Highlights, *currentHighlight)
			}

			// Start new highlight with timestamp format
			currentHighlight = &entities.Highlight{
				Time: matches[1],
				Page: 0, // Page info not available in this format
			}
			highlightText.Reset()
		} else if matches := pagePattern.FindStringSubmatch(line); matches != nil {
			// Save previous highlight if exists
			if currentHighlight != nil {
				currentHighlight.Text = strings.TrimSpace(highlightText.String())
				book.Highlights = append(book.Highlights, *currentHighlight)
			}

			// Start new highlight with page format
			page := 0
			// For now, just set page to 0 since we don't need exact page numbers for comparison
			currentHighlight = &entities.Highlight{
				Time: "unknown", // No timestamp in this format
				Page: page,
			}
			highlightText.Reset()
		} else if strings.HasPrefix(line, "## ") {
			// Skip section headers like "## Highlights:"
			continue
		} else if strings.HasPrefix(line, "---") {
			// Skip separator lines
			continue
		} else if currentHighlight != nil && line != "" {
			// Add content to current highlight
			if highlightText.Len() > 0 {
				highlightText.WriteString("\n")
			}
			highlightText.WriteString(line)
		}
	}

	// Save the last highlight
	if currentHighlight != nil {
		currentHighlight.Text = strings.TrimSpace(highlightText.String())
		book.Highlights = append(book.Highlights, *currentHighlight)
	}

	return nil
}

// CompareWithDatabase compares parsed books with database entries
func (parser *MarkdownParser) CompareWithDatabase(markdownBooks []entities.Book, dbBooks []entities.Book) ComparisonResult {
	result := ComparisonResult{
		MarkdownBooks:  len(markdownBooks),
		DatabaseBooks:  len(dbBooks),
		Matches:        make([]BookMatch, 0),
		OnlyInMarkdown: make([]entities.Book, 0),
		OnlyInDatabase: make([]entities.Book, 0),
	}

	// Create maps for easier lookup
	dbBookMap := make(map[string]entities.Book)
	for _, book := range dbBooks {
		key := generateBookKey(book.Title, book.Author)
		dbBookMap[key] = book
	}

	markdownBookMap := make(map[string]entities.Book)
	for _, book := range markdownBooks {
		key := generateBookKey(book.Title, book.Author)
		markdownBookMap[key] = book
	}

	// Find matches and markdown-only books
	for _, mdBook := range markdownBooks {
		key := generateBookKey(mdBook.Title, mdBook.Author)
		if dbBook, exists := dbBookMap[key]; exists {
			// Found a match
			match := BookMatch{
				Title:              mdBook.Title,
				Author:             mdBook.Author,
				MarkdownHighlights: len(mdBook.Highlights),
				DatabaseHighlights: len(dbBook.Highlights),
				HighlightsDiff:     len(mdBook.Highlights) - len(dbBook.Highlights),
			}
			result.Matches = append(result.Matches, match)
		} else {
			// Only in markdown
			result.OnlyInMarkdown = append(result.OnlyInMarkdown, mdBook)
		}
	}

	// Find database-only books
	for _, dbBook := range dbBooks {
		key := generateBookKey(dbBook.Title, dbBook.Author)
		if _, exists := markdownBookMap[key]; !exists {
			result.OnlyInDatabase = append(result.OnlyInDatabase, dbBook)
		}
	}

	return result
}

// ComparisonResult contains the results of comparing markdown and database books
type ComparisonResult struct {
	MarkdownBooks  int             `json:"markdown_books"`
	DatabaseBooks  int             `json:"database_books"`
	Matches        []BookMatch     `json:"matches"`
	OnlyInMarkdown []entities.Book `json:"only_in_markdown"`
	OnlyInDatabase []entities.Book `json:"only_in_database"`
}

// BookMatch represents a book that exists in both markdown and database
type BookMatch struct {
	Title              string `json:"title"`
	Author             string `json:"author"`
	MarkdownHighlights int    `json:"markdown_highlights"`
	DatabaseHighlights int    `json:"database_highlights"`
	HighlightsDiff     int    `json:"highlights_diff"`
}

// generateBookKey creates a consistent key for book identification
func generateBookKey(title, author string) string {
	return strings.ToLower(strings.TrimSpace(title)) + "|" + strings.ToLower(strings.TrimSpace(author))
}

// GetMarkdownFilePath returns the expected markdown file path for a book
func (parser *MarkdownParser) GetMarkdownFilePath(book entities.Book) string {
	return filepath.Join(parser.ExportDir, book.Title+".md")
}

// BookExists checks if a markdown file exists for the given book
func (parser *MarkdownParser) BookExists(book entities.Book) bool {
	filePath := parser.GetMarkdownFilePath(book)
	_, err := os.Stat(filePath)
	return err == nil
}
