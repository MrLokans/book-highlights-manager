package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/moonreader"
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
	fs.StringVar(&cmd.DatabasePath, "db", "./moonreader.db", "Path to the local database file for highlights")
	fs.StringVar(&cmd.TokenDatabasePath, "token-db", "./assistant.db", "Path to the database containing OAuth tokens")
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

	// Try to load token from database if not provided explicitly
	if !cmd.ExportOnly && cmd.DropboxToken == "" {
		if token, err := cmd.loadTokenFromStore(); err == nil && token != "" {
			cmd.DropboxToken = token
		}
	}

	// Validate token is provided (unless export-only mode)
	if !cmd.ExportOnly && cmd.DropboxToken == "" {
		return fmt.Errorf("Dropbox access token required. Either:\n  - Set DROPBOX_ACCESS_TOKEN environment variable\n  - Use -token flag\n  - Run '%s dropbox-auth' to authenticate and save tokens", os.Args[0])
	}

	return nil
}

// loadTokenFromStore attempts to load the Dropbox token from the encrypted store
func (cmd *MoonReaderDropboxCommand) loadTokenFromStore() (string, error) {
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: cmd.TokenDatabasePath,
	})
	if err != nil {
		return "", err
	}
	defer store.Close()

	token, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", nil
	}

	// Check if token is expired and needs refresh
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		// Token expired, try to refresh
		if token.RefreshToken != "" {
			fmt.Println("🔄 Access token expired, refreshing...")
			newToken, err := cmd.refreshToken(token.RefreshToken)
			if err != nil {
				return "", fmt.Errorf("failed to refresh token: %w", err)
			}

			// Update token in store
			var expiresAt *time.Time
			if newToken.expiresIn > 0 {
				exp := time.Now().Add(time.Duration(newToken.expiresIn) * time.Second)
				expiresAt = &exp
			}
			if err := store.UpdateTokenAfterRefresh(entities.OAuthProviderDropbox, token.AccountID, newToken.accessToken, "", expiresAt); err != nil {
				fmt.Printf("⚠️  Warning: Failed to update refreshed token in database: %v\n", err)
			}

			return newToken.accessToken, nil
		}
		return "", fmt.Errorf("token expired and no refresh token available")
	}

	fmt.Printf("🔑 Using stored token for account: %s\n", token.AccountID)
	return token.AccessToken, nil
}

type refreshedToken struct {
	accessToken string
	expiresIn   int
}

// refreshToken exchanges a refresh token for a new access token
func (cmd *MoonReaderDropboxCommand) refreshToken(refreshToken string) (*refreshedToken, error) {
	// Note: This requires the app key to be available
	appKey := os.Getenv("DROPBOX_APP_KEY")
	if appKey == "" {
		return nil, fmt.Errorf("DROPBOX_APP_KEY environment variable required for token refresh")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", appKey)

	req, err := http.NewRequest("POST", "https://api.dropboxapi.com/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	fmt.Println("✅ Token refreshed successfully")
	return &refreshedToken{
		accessToken: tokenResp.AccessToken,
		expiresIn:   tokenResp.ExpiresIn,
	}, nil
}

// Run executes the Dropbox sync command
func (cmd *MoonReaderDropboxCommand) Run() error {
	fmt.Println("🌙 MoonReader Dropbox Sync")
	fmt.Println("===========================")

	// Handle list-all mode (debugging)
	if cmd.ListAll {
		return cmd.listAllFiles()
	}

	// Handle list-only mode
	if cmd.ListOnly {
		return cmd.listBackups()
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

	fmt.Printf("📁 Database: %s\n", cmd.DatabasePath)
	fmt.Printf("📁 Output: %s\n", cmd.OutputDir)

	// Import from Dropbox unless export-only mode
	if !cmd.ExportOnly {
		if err := cmd.importFromDropbox(accessor); err != nil {
			return err
		}
	} else {
		fmt.Println("\n⏭️  Skipping Dropbox import (export-only mode)")
	}

	// Export to markdown
	if err := cmd.exportToMarkdown(accessor); err != nil {
		return err
	}

	fmt.Println("\n✅ Sync complete!")
	return nil
}

func (cmd *MoonReaderDropboxCommand) listAllFiles() error {
	pathDisplay := cmd.DropboxPath
	if pathDisplay == "" {
		pathDisplay = "(root)"
	}
	fmt.Printf("📂 Listing ALL entries in Dropbox path: %s\n\n", pathDisplay)

	client := moonreader.NewDropboxClient(cmd.DropboxToken)
	client.WithBasePath(cmd.DropboxPath)

	entries, err := client.ListAllEntries()
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No files or folders found.")
		fmt.Println("\n💡 Tips:")
		fmt.Println("  - If using 'App folder' access, the root is your app's folder")
		fmt.Println("  - Try: -dropbox-path=\"\" to list the root")
		fmt.Println("  - Check your app permissions at https://www.dropbox.com/developers/apps")
		return nil
	}

	fmt.Printf("Found %d entries:\n\n", len(entries))
	for _, entry := range entries {
		icon := "📄"
		if entry.Tag == "folder" {
			icon = "📁"
		}
		fmt.Printf("  %s %s\n", icon, entry.Name)
		fmt.Printf("     Type: %s\n", entry.Tag)
		fmt.Printf("     Path: %s\n", entry.PathDisplay)
		if entry.Tag == "file" {
			fmt.Printf("     Size: %d bytes\n", entry.Size)
			fmt.Printf("     Modified: %s\n", entry.ServerModified.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}

	return nil
}

func (cmd *MoonReaderDropboxCommand) listBackups() error {
	fmt.Printf("📂 Listing backups in Dropbox path: %s\n\n", cmd.DropboxPath)

	client := moonreader.NewDropboxClient(cmd.DropboxToken)
	client.WithBasePath(cmd.DropboxPath)

	files, err := client.ListBackupFiles()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No backup files found.")
		return nil
	}

	fmt.Printf("Found %d backup file(s):\n\n", len(files))
	for _, file := range files {
		fmt.Printf("  📄 %s\n", file.Name)
		fmt.Printf("     Path: %s\n", file.PathDisplay)
		fmt.Printf("     Size: %d bytes\n", file.Size)
		fmt.Printf("     Modified: %s\n\n", file.ServerModified.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func (cmd *MoonReaderDropboxCommand) importFromDropbox(accessor *moonreader.LocalDBAccessor) error {
	fmt.Println("\n☁️  Importing from Dropbox...")

	// Create Dropbox extractor
	extractor := moonreader.NewDropboxBackupExtractor(cmd.DropboxToken)
	extractor.WithBasePath(cmd.DropboxPath)

	// Download and extract latest backup
	fmt.Printf("🔍 Looking for backups in Dropbox: %s\n", cmd.DropboxPath)

	dbPath, cleanup, backupTime, err := extractor.ExtractLatestDatabase()
	if err != nil {
		return fmt.Errorf("failed to extract backup: %w", err)
	}
	defer cleanup()

	fmt.Printf("📥 Downloaded and extracted backup (modified: %s)\n",
		backupTime.Format("2006-01-02 15:04:05"))

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

func (cmd *MoonReaderDropboxCommand) exportToMarkdown(accessor *moonreader.LocalDBAccessor) error {
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
