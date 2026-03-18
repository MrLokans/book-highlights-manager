package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/kindle"
)

// KindleImportCommand handles importing highlights from Kindle My Clippings.txt
type KindleImportCommand struct {
	ClippingsPath  string
	DatabasePath   string
	OutputDir      string
	ExportMarkdown bool
	Verbose        bool
	DryRun         bool
}

// NewKindleImportCommand creates a CLI command for importing Kindle clippings.
func NewKindleImportCommand() *KindleImportCommand {
	return &KindleImportCommand{}
}

// ParseFlags parses command-line flags for the Kindle import.
func (cmd *KindleImportCommand) ParseFlags(args []string) error {
	fs := flag.NewFlagSet("kindle-import", flag.ExitOnError)

	fs.StringVar(&cmd.ClippingsPath, "file", "", "Path to Kindle 'My Clippings.txt' file (required)")
	fs.StringVar(&cmd.DatabasePath, "db", config.DefaultDatabasePath, "Path to the local database file for storing imported highlights")
	fs.StringVar(&cmd.OutputDir, "output", "", "Output directory for markdown files (if specified, exports to Obsidian-compatible markdown)")
	fs.BoolVar(&cmd.Verbose, "verbose", false, "Enable verbose logging")
	fs.BoolVar(&cmd.DryRun, "dry-run", false, "Show what would be imported without making changes")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s kindle-import -file <path> [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Import highlights from Kindle 'My Clippings.txt' to a local database.\n\n")
		fmt.Fprintf(os.Stderr, "The clippings file is typically found at:\n")
		fmt.Fprintf(os.Stderr, "  /Volumes/Kindle/documents/My Clippings.txt\n\n")
		fmt.Fprintf(os.Stderr, "By default, highlights are only saved to the database. Use -output to also\n")
		fmt.Fprintf(os.Stderr, "export them as Obsidian-compatible markdown files.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Import from connected Kindle device:\n")
		fmt.Fprintf(os.Stderr, "  %s kindle-import -file \"/Volumes/Kindle/documents/My Clippings.txt\"\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Import from local file and export to markdown:\n")
		fmt.Fprintf(os.Stderr, "  %s kindle-import -file \"My Clippings.txt\" -output ~/Obsidian/Highlights\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Preview what would be imported:\n")
		fmt.Fprintf(os.Stderr, "  %s kindle-import -file \"My Clippings.txt\" -dry-run -verbose\n", os.Args[0])
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if cmd.ClippingsPath == "" {
		return fmt.Errorf("required flag -file not provided")
	}

	cmd.ExportMarkdown = cmd.OutputDir != ""

	return nil
}

// Run executes the Kindle import workflow.
func (cmd *KindleImportCommand) Run() error {
	fmt.Println("Kindle Import")
	fmt.Println("=============")

	if cmd.DryRun {
		fmt.Println("DRY RUN MODE - No changes will be made")
		fmt.Println()
	}

	// Verify clippings file exists
	if _, err := os.Stat(cmd.ClippingsPath); os.IsNotExist(err) {
		return fmt.Errorf("clippings file not found: %s", cmd.ClippingsPath)
	}

	fmt.Printf("File: %s\n", cmd.ClippingsPath)

	// Open and parse clippings file
	fmt.Println("\nReading highlights from Kindle clippings...")

	file, err := os.Open(cmd.ClippingsPath)
	if err != nil {
		return fmt.Errorf("failed to open clippings file: %w", err)
	}
	defer file.Close()

	parser := kindle.NewParser()
	books, err := parser.Parse(file)
	if err != nil {
		return fmt.Errorf("failed to parse clippings: %w", err)
	}

	if len(books) == 0 {
		fmt.Println("No books with highlights found in clippings file")
		return nil
	}

	// Count total highlights
	totalHighlights := 0
	for _, book := range books {
		totalHighlights += len(book.Highlights)
	}

	fmt.Printf("Found %d books with %d total highlights\n", len(books), totalHighlights)

	if cmd.Verbose {
		fmt.Println("\n=== Books Found ===")
		for i, book := range books {
			authorStr := book.Author
			if authorStr == "" {
				authorStr = "(no author)"
			}
			fmt.Printf("%d. \"%s\" by %s (%d highlights)\n",
				i+1, book.Title, authorStr, len(book.Highlights))
		}
	}

	if cmd.DryRun {
		fmt.Println("\nDry run complete. Use without -dry-run to import.")
		return nil
	}

	// Convert database path to absolute
	absDBPath, err := filepath.Abs(cmd.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for database: %w", err)
	}
	cmd.DatabasePath = absDBPath

	fmt.Printf("\nSaving to database: %s\n", cmd.DatabasePath)

	// Initialize database
	db, err := database.NewDatabase(cmd.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	importBooksToDatabase(db, books, cmd.Verbose)

	if cmd.ExportMarkdown {
		if err := cmd.exportToMarkdown(books); err != nil {
			return err
		}
	}

	fmt.Println("\nImport complete!")
	return nil
}

func (cmd *KindleImportCommand) exportToMarkdown(books []entities.Book) error {
	absOutputDir, err := filepath.Abs(cmd.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	fmt.Printf("\nExporting to markdown: %s\n", absOutputDir)
	mdExporter := exporters.NewMarkdownExporter(absOutputDir)
	result, err := mdExporter.Export(books)
	if err != nil {
		return fmt.Errorf("failed to export to markdown: %w", err)
	}

	fmt.Printf("Exported %d books to markdown\n", result.BooksProcessed)
	if result.BooksFailed > 0 {
		fmt.Printf("%d books failed to export\n", result.BooksFailed)
	}
	return nil
}
