package cli

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
	booksdb "github.com/mrlokans/assistant/internal/database/books"
	"github.com/mrlokans/assistant/internal/database/sources"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/parsers"
)

// ParseMarkdownCommand imports highlights from exported markdown files back into the database.
type ParseMarkdownCommand struct {
	Directory    string
	DatabasePath string
	CompareDB    bool
	Verbose      bool
}

// NewParseMarkdownCommand creates a CLI command for parsing markdown files.
func NewParseMarkdownCommand() *ParseMarkdownCommand {
	return &ParseMarkdownCommand{}
}

// ParseFlags parses command-line flags for the markdown parser.
func (cmd *ParseMarkdownCommand) ParseFlags(args []string) error {
	fs := flag.NewFlagSet("parse-markdown", flag.ExitOnError)

	fs.StringVar(&cmd.Directory, "dir", "", "Directory to recursively search for markdown files (required)")
	fs.StringVar(&cmd.DatabasePath, "db", config.DefaultDatabasePath, "Path to the database file for comparison")
	fs.BoolVar(&cmd.CompareDB, "compare", false, "Compare parsed books with database entries")
	fs.BoolVar(&cmd.Verbose, "verbose", false, "Enable verbose logging")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s parse-markdown [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Parse markdown files recursively from a directory and optionally compare with database.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s parse-markdown -dir ./exported-books\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s parse-markdown -dir ./exported-books -compare -db ./my-books.db\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s parse-markdown -dir ./exported-books -verbose\n", os.Args[0])
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if cmd.Directory == "" {
		fs.Usage()
		return fmt.Errorf("directory is required")
	}

	return nil
}

// Run executes the markdown parsing workflow.
func (cmd *ParseMarkdownCommand) Run() error {
	// Validate directory exists
	if _, err := os.Stat(cmd.Directory); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", cmd.Directory)
	}

	// Convert to absolute path for cleaner output
	absDir, err := filepath.Abs(cmd.Directory)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd.Directory = absDir

	fmt.Printf("Parsing markdown files from directory: %s\n", cmd.Directory)
	if cmd.Verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	// Create parser
	parser := parsers.NewMarkdownParser(cmd.Directory)

	// Parse all markdown files recursively
	books, result, err := parser.ParseAllMarkdownFilesRecursive(cmd.Directory)
	if err != nil {
		return fmt.Errorf("failed to parse markdown files: %w", err)
	}

	// Print results
	fmt.Printf("\n=== Parsing Results ===\n")
	fmt.Printf("Books processed: %d\n", result.BooksProcessed)
	fmt.Printf("Highlights processed: %d\n", result.HighlightsProcessed)
	fmt.Printf("Books failed: %d\n", result.BooksFailed)

	if result.BooksProcessed > 0 {
		fmt.Printf("\n=== Parsed Books ===\n")
		for i, book := range books {
			fmt.Printf("%d. \"%s\" by %s (%d highlights)\n",
				i+1, book.Title, book.Author, len(book.Highlights))
		}
	}

	// Compare with database if requested
	if cmd.CompareDB {
		fmt.Printf("\n=== Database Comparison ===\n")
		if err := cmd.compareWithDatabase(books); err != nil {
			return fmt.Errorf("failed to compare with database: %w", err)
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	if result.BooksFailed > 0 {
		fmt.Printf("⚠️  %d files failed to parse (check logs above for details)\n", result.BooksFailed)
	}
	if result.BooksProcessed > 0 {
		fmt.Printf("✅ Successfully parsed %d books with %d total highlights\n",
			result.BooksProcessed, result.HighlightsProcessed)
	} else {
		fmt.Printf("ℹ️  No valid book markdown files found in the directory\n")
	}

	return nil
}

func (cmd *ParseMarkdownCommand) compareWithDatabase(markdownBooks []entities.Book) error {
	// Check if database exists
	if _, err := os.Stat(cmd.DatabasePath); os.IsNotExist(err) {
		fmt.Printf("Database file does not exist: %s\n", cmd.DatabasePath)
		fmt.Printf("Skipping database comparison.\n")
		return nil
	}

	// Initialize database
	db, err := database.NewDatabase(cmd.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Error closing database", "error", err)
		}
	}()

	// Get all books from database
	sourcesRepo := sources.NewRepository(db.DB)
	booksRepo := booksdb.NewRepository(db.DB, sourcesRepo)
	dbBooks, err := booksRepo.GetAllBooks()
	if err != nil {
		return fmt.Errorf("failed to get books from database: %w", err)
	}

	fmt.Printf("Database contains %d books\n", len(dbBooks))

	// Create parser for comparison (we need the comparison methods)
	parser := parsers.NewMarkdownParser(cmd.Directory)

	// Compare the two sets
	comparisonResult := parser.CompareWithDatabase(markdownBooks, dbBooks)

	printComparisonResult(comparisonResult)
	return nil
}

func printComparisonResult(r parsers.ComparisonResult) {
	fmt.Printf("\n=== Comparison Results ===\n")
	fmt.Printf("Books in markdown: %d\n", r.MarkdownBooks)
	fmt.Printf("Books in database: %d\n", r.DatabaseBooks)
	fmt.Printf("Matches: %d\n", len(r.Matches))
	fmt.Printf("Only in markdown: %d\n", len(r.OnlyInMarkdown))
	fmt.Printf("Only in database: %d\n", len(r.OnlyInDatabase))

	for i, match := range r.Matches {
		if i == 0 {
			fmt.Printf("\n=== Matched Books ===\n")
		}
		diff := ""
		switch {
		case match.HighlightsDiff > 0:
			diff = fmt.Sprintf(" (+%d highlights in markdown)", match.HighlightsDiff)
		case match.HighlightsDiff < 0:
			diff = fmt.Sprintf(" (%d highlights in markdown)", match.HighlightsDiff)
		}
		fmt.Printf("%d. \"%s\" by %s (MD: %d, DB: %d)%s\n",
			i+1, match.Title, match.Author,
			match.MarkdownHighlights, match.DatabaseHighlights, diff)
	}

	for i, book := range r.OnlyInMarkdown {
		if i == 0 {
			fmt.Printf("\n=== Only in Markdown ===\n")
		}
		fmt.Printf("%d. \"%s\" by %s (%d highlights)\n",
			i+1, book.Title, book.Author, len(book.Highlights))
	}

	for i, book := range r.OnlyInDatabase {
		if i == 0 {
			fmt.Printf("\n=== Only in Database ===\n")
		}
		fmt.Printf("%d. \"%s\" by %s (%d highlights)\n",
			i+1, book.Title, book.Author, len(book.Highlights))
	}
}
