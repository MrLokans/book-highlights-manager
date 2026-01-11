package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mrlokans/assistant/internal/applebooks"
	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/exporters"
)

// AppleBooksImportCommand handles importing highlights from Apple Books
type AppleBooksImportCommand struct {
	AnnotationDBPath string
	BookDBPath       string
	DatabasePath     string
	OutputDir        string
	ExportMarkdown   bool // true if -output was explicitly specified
	Verbose          bool
	DryRun           bool
}

// NewAppleBooksImportCommand creates a new AppleBooksImportCommand
func NewAppleBooksImportCommand() *AppleBooksImportCommand {
	return &AppleBooksImportCommand{}
}

// ParseFlags parses command line flags
func (cmd *AppleBooksImportCommand) ParseFlags(args []string) error {
	fs := flag.NewFlagSet("applebooks-import", flag.ExitOnError)

	fs.StringVar(&cmd.AnnotationDBPath, "annotation-db", "", "Path to Apple Books annotation database (auto-detected if not specified)")
	fs.StringVar(&cmd.BookDBPath, "book-db", "", "Path to Apple Books library database (auto-detected if not specified)")
	fs.StringVar(&cmd.DatabasePath, "db", config.DefaultDatabasePath, "Path to the local database file for storing imported highlights")
	fs.StringVar(&cmd.OutputDir, "output", "", "Output directory for markdown files (if specified, exports to Obsidian-compatible markdown)")
	fs.BoolVar(&cmd.Verbose, "verbose", false, "Enable verbose logging")
	fs.BoolVar(&cmd.DryRun, "dry-run", false, "Show what would be imported without making changes")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s applebooks-import [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Import highlights from Apple Books to a local database.\n\n")
		fmt.Fprintf(os.Stderr, "By default, highlights are only saved to the database. Use -output to also\n")
		fmt.Fprintf(os.Stderr, "export them as Obsidian-compatible markdown files.\n\n")

		if runtime.GOOS == "darwin" {
			fmt.Fprintf(os.Stderr, "On macOS, the Apple Books database paths are automatically detected:\n")
			fmt.Fprintf(os.Stderr, "  - Annotations: ~/Library/Containers/com.apple.iBooksX/Data/Documents/AEAnnotation/\n")
			fmt.Fprintf(os.Stderr, "  - Books: ~/Library/Containers/com.apple.iBooksX/Data/Documents/BKLibrary/\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "NOTE: Apple Books is only available on macOS. You can still import from\n")
			fmt.Fprintf(os.Stderr, "exported database files using the -annotation-db and -book-db flags.\n\n")
		}

		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Import to database only (auto-detect Apple Books databases on macOS):\n")
		fmt.Fprintf(os.Stderr, "  %s applebooks-import\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Import to database and export to markdown:\n")
		fmt.Fprintf(os.Stderr, "  %s applebooks-import -output ~/Obsidian/Highlights\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Specify custom database paths:\n")
		fmt.Fprintf(os.Stderr, "  %s applebooks-import -annotation-db /path/to/AEAnnotation.sqlite -book-db /path/to/BKLibrary.sqlite\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Preview what would be imported:\n")
		fmt.Fprintf(os.Stderr, "  %s applebooks-import -dry-run -verbose\n", os.Args[0])
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if -output was explicitly specified
	cmd.ExportMarkdown = cmd.OutputDir != ""

	return nil
}

// Run executes the import command
func (cmd *AppleBooksImportCommand) Run() error {
	fmt.Println("📚 Apple Books Import")
	fmt.Println("=====================")

	if cmd.DryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes will be made")
		fmt.Println()
	}

	// Create Apple Books reader
	reader, err := applebooks.NewAppleBooksReader(cmd.AnnotationDBPath, cmd.BookDBPath)
	if err != nil {
		return fmt.Errorf("failed to create Apple Books reader: %w", err)
	}

	fmt.Printf("📁 Annotation DB: %s\n", reader.GetAnnotationDBPath())
	fmt.Printf("📁 Book DB: %s\n", reader.GetBookDBPath())

	// Read highlights
	fmt.Println("\n📖 Reading highlights from Apple Books...")
	books, err := reader.GetBooks()
	if err != nil {
		return fmt.Errorf("failed to read highlights: %w", err)
	}

	if len(books) == 0 {
		fmt.Println("ℹ️  No books with highlights found in Apple Books")
		return nil
	}

	// Count total highlights
	totalHighlights := 0
	for _, book := range books {
		totalHighlights += len(book.Highlights)
	}

	fmt.Printf("📚 Found %d books with %d total highlights\n", len(books), totalHighlights)

	if cmd.Verbose {
		fmt.Println("\n=== Books Found ===")
		for i, book := range books {
			fmt.Printf("%d. \"%s\" by %s (%d highlights)\n",
				i+1, book.Title, book.Author, len(book.Highlights))
		}
	}

	if cmd.DryRun {
		fmt.Println("\n✅ Dry run complete. Use without -dry-run to import.")
		return nil
	}

	// Convert database path to absolute
	absDBPath, err := filepath.Abs(cmd.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for database: %w", err)
	}
	cmd.DatabasePath = absDBPath

	fmt.Printf("\n💾 Saving to database: %s\n", cmd.DatabasePath)

	// Initialize database
	db, err := database.NewDatabase(cmd.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Import all books to database
	fmt.Println("\n📥 Importing books to database...")

	var importedBooks, importedHighlights int
	var importErrors []string

	for _, book := range books {
		if cmd.Verbose {
			fmt.Printf("  → \"%s\" by %s (%d highlights)...\n",
				book.Title, book.Author, len(book.Highlights))
		}

		if err := db.SaveBook(&book); err != nil {
			errMsg := fmt.Sprintf("Failed to save \"%s\": %v", book.Title, err)
			importErrors = append(importErrors, errMsg)
			if cmd.Verbose {
				fmt.Printf("    ❌ %s\n", err)
			}
			continue
		}

		importedBooks++
		importedHighlights += len(book.Highlights)

		if cmd.Verbose {
			fmt.Printf("    ✅ Saved\n")
		}
	}

	// Print database import summary
	fmt.Println("\n=== Database Import Summary ===")
	fmt.Printf("📚 Books saved: %d/%d\n", importedBooks, len(books))
	fmt.Printf("📝 Highlights saved: %d\n", importedHighlights)

	if len(importErrors) > 0 {
		fmt.Printf("\n⚠️  %d errors occurred:\n", len(importErrors))
		for _, errMsg := range importErrors {
			fmt.Printf("  ❌ %s\n", errMsg)
		}
	}

	// Export to markdown if -output was specified
	if cmd.ExportMarkdown {
		absOutputDir, err := filepath.Abs(cmd.OutputDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for output: %w", err)
		}
		cmd.OutputDir = absOutputDir

		fmt.Printf("\n📝 Exporting to markdown: %s\n", cmd.OutputDir)

		// Create markdown exporter
		mdExporter := exporters.NewMarkdownExporter(cmd.OutputDir, "")

		result, err := mdExporter.Export(books)
		if err != nil {
			return fmt.Errorf("failed to export to markdown: %w", err)
		}

		fmt.Printf("📄 Exported %d books to markdown\n", result.BooksProcessed)
		if result.BooksFailed > 0 {
			fmt.Printf("⚠️  %d books failed to export\n", result.BooksFailed)
		}
	}

	fmt.Println("\n✅ Import complete!")
	return nil
}
