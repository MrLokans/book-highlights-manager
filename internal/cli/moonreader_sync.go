package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrlokans/assistant/internal/moonreader"
)

// MoonReaderSyncCommand handles syncing MoonReader highlights
type MoonReaderSyncCommand struct {
	BackupDir    string
	DatabasePath string
	OutputDir    string
	Verbose      bool
	ExportOnly   bool
}

// NewMoonReaderSyncCommand creates a new MoonReaderSyncCommand
func NewMoonReaderSyncCommand() *MoonReaderSyncCommand {
	return &MoonReaderSyncCommand{}
}

// ParseFlags parses command line flags
func (cmd *MoonReaderSyncCommand) ParseFlags(args []string) error {
	fs := flag.NewFlagSet("moonreader-sync", flag.ExitOnError)

	homeDir, _ := os.UserHomeDir()
	defaultBackupDir := filepath.Join(homeDir, "syncthing", "one-plus", "moonreader", "Backup")
	defaultOutputDir := filepath.Join(".", "markdown")

	fs.StringVar(&cmd.BackupDir, "backup-dir", defaultBackupDir, "Directory containing MoonReader backup files")
	fs.StringVar(&cmd.DatabasePath, "db", "./moonreader.db", "Path to the local database file")
	fs.StringVar(&cmd.OutputDir, "output", defaultOutputDir, "Output directory for markdown files")
	fs.BoolVar(&cmd.Verbose, "verbose", false, "Enable verbose logging")
	fs.BoolVar(&cmd.ExportOnly, "export-only", false, "Only export existing notes (skip backup import)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s moonreader-sync [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Sync MoonReader highlights from backup files to Obsidian-compatible markdown.\n\n")
		fmt.Fprintf(os.Stderr, "This command:\n")
		fmt.Fprintf(os.Stderr, "  1. Finds the latest MoonReader backup (.mrpro/.mrstd)\n")
		fmt.Fprintf(os.Stderr, "  2. Extracts and imports highlights to local database\n")
		fmt.Fprintf(os.Stderr, "  3. Exports all books as markdown files with Obsidian callouts\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s moonreader-sync\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s moonreader-sync -output ~/Obsidian/Highlights\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s moonreader-sync -export-only\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s moonreader-sync -backup-dir /path/to/backups -verbose\n", os.Args[0])
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	return nil
}

// Run executes the sync command
func (cmd *MoonReaderSyncCommand) Run() error {
	fmt.Println("🌙 MoonReader Sync")
	fmt.Println("==================")

	// Convert paths to absolute
	absOutputDir, err := filepath.Abs(cmd.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}
	cmd.OutputDir = absOutputDir

	absDBPath, err := filepath.Abs(cmd.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for database: %w", err)
	}
	cmd.DatabasePath = absDBPath

	// Initialize local database
	accessor, err := moonreader.NewLocalDBAccessor(cmd.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize local database: %w", err)
	}
	defer accessor.Close()

	fmt.Printf("📁 Database: %s\n", cmd.DatabasePath)
	fmt.Printf("📁 Output: %s\n", cmd.OutputDir)

	// Import from backup unless export-only mode
	if !cmd.ExportOnly {
		if err := cmd.importFromBackup(accessor); err != nil {
			return err
		}
	} else {
		fmt.Println("\n⏭️  Skipping backup import (export-only mode)")
	}

	// Export to markdown
	if err := cmd.exportToMarkdown(accessor); err != nil {
		return err
	}

	fmt.Println("\n✅ Sync complete!")
	return nil
}

func (cmd *MoonReaderSyncCommand) importFromBackup(accessor *moonreader.LocalDBAccessor) error {
	fmt.Println("\n📦 Importing from MoonReader backup...")

	// Create backup extractor
	extractor := moonreader.NewBackupExtractor(cmd.BackupDir)

	// Find and extract latest backup
	fmt.Printf("🔍 Looking for backups in: %s\n", cmd.BackupDir)

	backup, err := extractor.FindLatestBackup()
	if err != nil {
		return fmt.Errorf("failed to find backup: %w", err)
	}

	fmt.Printf("📄 Found backup: %s (modified: %s)\n",
		filepath.Base(backup.Path),
		backup.ModTime.Format("2006-01-02 15:04:05"))

	// Extract database
	dbPath, tempDir, err := extractor.ExtractDatabase(backup.Path)
	if err != nil {
		return fmt.Errorf("failed to extract backup: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	fmt.Printf("📂 Extracted database to: %s\n", dbPath)

	// Read notes from backup
	reader := moonreader.NewBackupDBReader(dbPath)
	notes, err := reader.GetNotes()
	if err != nil {
		return fmt.Errorf("failed to read notes from backup: %w", err)
	}

	fmt.Printf("📝 Found %d highlights in backup\n", len(notes))

	if len(notes) == 0 {
		fmt.Println("⚠️  No highlights found in backup")
		return nil
	}

	// Upsert notes to local database
	if err := accessor.UpsertNotes(notes); err != nil {
		return fmt.Errorf("failed to save notes: %w", err)
	}

	fmt.Printf("💾 Saved %d highlights to local database\n", len(notes))

	// Group by book for summary
	bookCount := make(map[string]int)
	for _, note := range notes {
		bookCount[note.BookTitle]++
	}
	fmt.Printf("📚 Highlights from %d books\n", len(bookCount))

	if cmd.Verbose {
		fmt.Println("\n=== Books with highlights ===")
		for title, count := range bookCount {
			fmt.Printf("  - %s: %d highlights\n", title, count)
		}
	}

	return nil
}

func (cmd *MoonReaderSyncCommand) exportToMarkdown(accessor *moonreader.LocalDBAccessor) error {
	fmt.Println("\n📤 Exporting to Obsidian markdown...")

	exporter := moonreader.NewObsidianExporter(cmd.OutputDir, accessor)
	result, err := exporter.Export()
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	if len(result.ExportedFiles) == 0 {
		fmt.Println("ℹ️  No books to export")
		return nil
	}

	fmt.Printf("✅ Exported %d books:\n", len(result.ExportedFiles))

	for title, path := range result.ExportedFiles {
		fmt.Printf("  📖 %s → %s\n", title, filepath.Base(path))
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\n⚠️  %d errors during export:\n", len(result.Errors))
		for _, errMsg := range result.Errors {
			fmt.Printf("  ❌ %s\n", errMsg)
		}
	}

	return nil
}
