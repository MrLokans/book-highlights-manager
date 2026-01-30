package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/moonreader"
	"github.com/mrlokans/assistant/internal/oauth2"
	"github.com/mrlokans/assistant/internal/oauth2/providers"
	"github.com/mrlokans/assistant/internal/storage"
	dropboxstorage "github.com/mrlokans/assistant/internal/storage/providers/dropbox"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

// MoonReaderDropboxCommand handles syncing MoonReader highlights from Dropbox
type MoonReaderDropboxCommand struct {
	DropboxToken      string
	DropboxPath       string
	DatabasePath      string
	TokenDatabasePath string
	OutputDir         string
	Verbose           bool
	ExportOnly        bool
	ListOnly          bool
	ListAll           bool
}

// NewMoonReaderDropboxCommand creates a new MoonReaderDropboxCommand
func NewMoonReaderDropboxCommand() *MoonReaderDropboxCommand {
	return &MoonReaderDropboxCommand{}
}

// ParseFlags parses command line flags
func (cmd *MoonReaderDropboxCommand) ParseFlags(args []string) error {
	fs := flag.NewFlagSet("moonreader-dropbox", flag.ExitOnError)

	defaultOutputDir := filepath.Join(".", "markdown")
	defaultDropboxPath := "/Apps/Books/.Moon+/Backup"

	// Token can come from env or flag
	envToken := os.Getenv("DROPBOX_ACCESS_TOKEN")

	fs.StringVar(&cmd.DropboxToken, "token", envToken, "Dropbox access token (or set DROPBOX_ACCESS_TOKEN env variable)")
	fs.StringVar(&cmd.DropboxPath, "dropbox-path", defaultDropboxPath, "Path to MoonReader backups in Dropbox")
	fs.StringVar(&cmd.DatabasePath, "db", config.DefaultMoonReaderDatabasePath, "Path to the local database file for highlights")
	fs.StringVar(&cmd.TokenDatabasePath, "token-db", config.DefaultDatabasePath, "Path to the database containing OAuth tokens")
	fs.StringVar(&cmd.OutputDir, "output", defaultOutputDir, "Output directory for markdown files")
	fs.BoolVar(&cmd.Verbose, "verbose", false, "Enable verbose logging")
	fs.BoolVar(&cmd.ExportOnly, "export-only", false, "Only export existing notes (skip Dropbox import)")
	fs.BoolVar(&cmd.ListOnly, "list", false, "Only list available backup files in Dropbox")
	fs.BoolVar(&cmd.ListAll, "list-all", false, "List ALL files/folders in Dropbox path (for debugging access issues)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s moonreader-dropbox [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Sync MoonReader highlights from Dropbox backups to Obsidian-compatible markdown.\n\n")
		fmt.Fprintf(os.Stderr, "This command:\n")
		fmt.Fprintf(os.Stderr, "  1. Downloads the latest MoonReader backup from Dropbox\n")
		fmt.Fprintf(os.Stderr, "  2. Extracts and imports highlights to local database\n")
		fmt.Fprintf(os.Stderr, "  3. Exports all books as markdown files with Obsidian callouts\n\n")
		fmt.Fprintf(os.Stderr, "Authentication (in priority order):\n")
		fmt.Fprintf(os.Stderr, "  1. -token flag or DROPBOX_ACCESS_TOKEN environment variable\n")
		fmt.Fprintf(os.Stderr, "  2. Stored tokens from database (run 'dropbox-auth' first)\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Using environment variable\n")
		fmt.Fprintf(os.Stderr, "  export DROPBOX_ACCESS_TOKEN=your_token\n")
		fmt.Fprintf(os.Stderr, "  %s moonreader-dropbox\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Using command-line flag\n")
		fmt.Fprintf(os.Stderr, "  %s moonreader-dropbox -token=your_token\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # List available backups\n")
		fmt.Fprintf(os.Stderr, "  %s moonreader-dropbox -list\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Debug: List root folder (for App Folder apps, use empty path)\n")
		fmt.Fprintf(os.Stderr, "  %s moonreader-dropbox -list-all -dropbox-path=\"\"\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Custom output directory\n")
		fmt.Fprintf(os.Stderr, "  %s moonreader-dropbox -output ~/Obsidian/Highlights\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Dropbox App Types:\n")
		fmt.Fprintf(os.Stderr, "  - 'App folder' access: Use -dropbox-path=\"\" (app sees only its folder as root)\n")
		fmt.Fprintf(os.Stderr, "  - 'Full Dropbox' access: Use -dropbox-path=\"/Apps/Books/.Moon+/Backup\" (default)\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Note: Token validation happens in Run() - token may be loaded from store

	return nil
}

// Run executes the Dropbox sync command
func (cmd *MoonReaderDropboxCommand) Run() error {
	fmt.Println("MoonReader Dropbox Sync")
	fmt.Println("=======================")

	// Get token source - either from direct token or from store
	tokenSource, err := cmd.getTokenSource()
	if err != nil && !cmd.ExportOnly {
		return err
	}

	// Handle list-all mode (debugging)
	if cmd.ListAll {
		return cmd.listAllFiles(tokenSource)
	}

	// Handle list-only mode
	if cmd.ListOnly {
		return cmd.listBackups(tokenSource)
	}

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

	fmt.Printf("Database: %s\n", cmd.DatabasePath)
	fmt.Printf("Output: %s\n", cmd.OutputDir)

	// Import from Dropbox unless export-only mode
	if !cmd.ExportOnly {
		if tokenSource == nil {
			return fmt.Errorf("dropbox access token required: set DROPBOX_ACCESS_TOKEN environment variable, use -token flag, or run '%s dropbox-auth' to authenticate and save tokens", os.Args[0])
		}
		if err := cmd.importFromDropbox(tokenSource, accessor); err != nil {
			return err
		}
	} else {
		fmt.Println("\nSkipping Dropbox import (export-only mode)")
	}

	// Export to markdown
	if err := cmd.exportToMarkdown(accessor); err != nil {
		return err
	}

	fmt.Println("\nSync complete!")
	return nil
}

// getTokenSource returns a token source for Dropbox API access
func (cmd *MoonReaderDropboxCommand) getTokenSource() (oauth2.TokenSource, error) {
	// Priority 1: Direct token from flag or environment
	if cmd.DropboxToken != "" {
		return oauth2.NewStaticTokenSource(cmd.DropboxToken, ""), nil
	}

	// Priority 2: Token from encrypted store with auto-refresh
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: cmd.TokenDatabasePath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open token store: %w", err)
	}

	// Check if we have a token for Dropbox
	token, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	if token == nil {
		store.Close()
		return nil, nil // No token available
	}

	fmt.Printf("Using stored token for account: %s\n", token.AccountID)

	// Get the Dropbox app key for token refresh
	appKey := os.Getenv("DROPBOX_APP_KEY")
	if appKey == "" {
		// Token refresh won't work without app key, but we can still use the token
		fmt.Println("Warning: DROPBOX_APP_KEY not set, automatic token refresh disabled")
		return oauth2.NewStaticTokenSource(token.AccessToken, token.AccountID), nil
	}

	// Create provider and token source with auto-refresh
	provider := providers.NewDropboxProvider(appKey)
	return oauth2.NewStoredTokenSource(provider, store, token.AccountID), nil
}

func (cmd *MoonReaderDropboxCommand) listAllFiles(tokenSource oauth2.TokenSource) error {
	pathDisplay := cmd.DropboxPath
	if pathDisplay == "" {
		pathDisplay = "(root)"
	}
	fmt.Printf("Listing ALL entries in Dropbox path: %s\n\n", pathDisplay)

	ctx := context.Background()
	client := dropboxstorage.NewClient(tokenSource)

	entries, err := client.List(ctx, cmd.DropboxPath)
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No files or folders found.")
		fmt.Println("\nTips:")
		fmt.Println("  - If using 'App folder' access, the root is your app's folder")
		fmt.Println("  - Try: -dropbox-path=\"\" to list the root")
		fmt.Println("  - Check your app permissions at https://www.dropbox.com/developers/apps")
		return nil
	}

	fmt.Printf("Found %d entries:\n\n", len(entries))
	for _, entry := range entries {
		icon := "FILE"
		if entry.IsDir {
			icon = "DIR "
		}
		fmt.Printf("  %s %s\n", icon, entry.Name)
		fmt.Printf("       Path: %s\n", entry.Path)
		if !entry.IsDir {
			fmt.Printf("       Size: %d bytes\n", entry.Size)
			fmt.Printf("       Modified: %s\n", entry.ModifiedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}

	return nil
}

func (cmd *MoonReaderDropboxCommand) listBackups(tokenSource oauth2.TokenSource) error {
	fmt.Printf("Listing backups in Dropbox path: %s\n\n", cmd.DropboxPath)

	ctx := context.Background()
	client := dropboxstorage.NewClient(tokenSource)

	entries, err := client.List(ctx, cmd.DropboxPath)
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// Filter for backup files
	backupFiles := storage.FilterFiles(entries, isBackupFile)

	if len(backupFiles) == 0 {
		fmt.Println("No backup files found.")
		return nil
	}

	fmt.Printf("Found %d backup file(s):\n\n", len(backupFiles))
	for _, file := range backupFiles {
		fmt.Printf("  %s\n", file.Name)
		fmt.Printf("     Path: %s\n", file.Path)
		fmt.Printf("     Size: %d bytes\n", file.Size)
		fmt.Printf("     Modified: %s\n\n", file.ModifiedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func isBackupFile(f storage.FileInfo) bool {
	if f.IsDir {
		return false
	}
	name := f.Name
	return len(name) > 6 && (name[len(name)-6:] == ".mrstd" || name[len(name)-6:] == ".mrpro")
}

func (cmd *MoonReaderDropboxCommand) importFromDropbox(tokenSource oauth2.TokenSource, accessor *moonreader.LocalDBAccessor) error {
	fmt.Println("\nImporting from Dropbox...")

	ctx := context.Background()
	client := dropboxstorage.NewClient(tokenSource)

	// List backup files
	fmt.Printf("Looking for backups in Dropbox: %s\n", cmd.DropboxPath)
	entries, err := client.List(ctx, cmd.DropboxPath)
	if err != nil {
		return fmt.Errorf("failed to list folder: %w", err)
	}

	// Find backup files
	backupFiles := storage.FilterFiles(entries, isBackupFile)
	if len(backupFiles) == 0 {
		return fmt.Errorf("no backup files found in Dropbox path: %s", cmd.DropboxPath)
	}

	// Find latest backup
	latest := storage.FindLatest(backupFiles)
	if latest == nil {
		return fmt.Errorf("no backup files found")
	}

	fmt.Printf("Found latest backup: %s (modified: %s)\n",
		latest.Name, latest.ModifiedAt.Format("2006-01-02 15:04:05"))

	// Download the backup
	reader, err := client.Download(ctx, latest.Path)
	if err != nil {
		return fmt.Errorf("failed to download backup: %w", err)
	}
	defer reader.Close()

	// Create temp directory for extraction
	tempDir, err := os.MkdirTemp("", "moonreader-dropbox-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Save to temp file
	localPath := filepath.Join(tempDir, latest.Name)
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}

	_, err = localFile.ReadFrom(reader)
	localFile.Close()
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Downloaded backup to temp location\n")

	// Extract the database
	extractor := &moonreader.BackupExtractor{}
	dbPath, extractDir, err := extractor.ExtractDatabase(localPath)
	if err != nil {
		return fmt.Errorf("failed to extract database: %w", err)
	}
	defer os.RemoveAll(extractDir)

	// Read notes from backup
	dbReader := moonreader.NewBackupDBReader(dbPath)
	notes, err := dbReader.GetNotes()
	if err != nil {
		return fmt.Errorf("failed to read notes from backup: %w", err)
	}

	fmt.Printf("Found %d highlights in backup\n", len(notes))

	if len(notes) == 0 {
		fmt.Println("Warning: No highlights found in backup")
		return nil
	}

	// Upsert notes to local database
	if err := accessor.UpsertNotes(notes); err != nil {
		return fmt.Errorf("failed to save notes: %w", err)
	}

	fmt.Printf("Saved %d highlights to local database\n", len(notes))

	// Group by book for summary
	bookCount := make(map[string]int)
	for _, note := range notes {
		bookCount[note.BookTitle]++
	}
	fmt.Printf("Highlights from %d books\n", len(bookCount))

	if cmd.Verbose {
		fmt.Println("\n=== Books with highlights ===")
		for title, count := range bookCount {
			fmt.Printf("  - %s: %d highlights\n", title, count)
		}
	}

	return nil
}

func (cmd *MoonReaderDropboxCommand) exportToMarkdown(accessor *moonreader.LocalDBAccessor) error {
	fmt.Println("\nExporting to Obsidian markdown...")

	// Get notes grouped by book
	notesByBook, err := accessor.GetNotesByBook()
	if err != nil {
		return fmt.Errorf("failed to get notes: %w", err)
	}

	if len(notesByBook) == 0 {
		fmt.Println("No books to export")
		return nil
	}

	// Convert to entities
	books := moonreader.ConvertToEntities(notesByBook)

	// Use the main markdown exporter
	mdExporter := exporters.NewMarkdownExporter(cmd.OutputDir)
	result, err := mdExporter.Export(books)
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	fmt.Printf("Exported %d books with %d highlights\n", result.BooksProcessed, result.HighlightsProcessed)

	return nil
}
