package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/config"
	"github.com/mrlokans/assistant/internal/oauth2"
	"github.com/mrlokans/assistant/internal/oauth2/providers"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

// DropboxAuthCommand handles the Dropbox OAuth flow
type DropboxAuthCommand struct {
	AppKey       string
	RedirectURI  string
	Port         int
	Manual       bool
	DatabasePath string
	NoSave       bool
}

// NewDropboxAuthCommand creates a new DropboxAuthCommand
func NewDropboxAuthCommand() *DropboxAuthCommand {
	return &DropboxAuthCommand{}
}

// ParseFlags parses command line flags
func (cmd *DropboxAuthCommand) ParseFlags(args []string) error {
	fs := flag.NewFlagSet("dropbox-auth", flag.ExitOnError)

	// App key can come from env or flag
	envAppKey := os.Getenv("DROPBOX_APP_KEY")

	fs.StringVar(&cmd.AppKey, "app-key", envAppKey, "Dropbox App Key (or set DROPBOX_APP_KEY env variable)")
	fs.IntVar(&cmd.Port, "port", 8089, "Local port for OAuth callback server")
	fs.BoolVar(&cmd.Manual, "manual", false, "Use manual flow (copy/paste code instead of local server)")
	fs.StringVar(&cmd.DatabasePath, "db", config.DefaultDatabasePath, "Path to the database for storing tokens")
	fs.BoolVar(&cmd.NoSave, "no-save", false, "Don't save tokens to database (print only)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s dropbox-auth [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Perform Dropbox OAuth flow to obtain an access token.\n\n")
		fmt.Fprintf(os.Stderr, "This command uses the PKCE (Proof Key for Code Exchange) flow,\n")
		fmt.Fprintf(os.Stderr, "which is secure for CLI applications without needing an app secret.\n\n")
		fmt.Fprintf(os.Stderr, "Prerequisites:\n")
		fmt.Fprintf(os.Stderr, "  1. Create a Dropbox app at https://www.dropbox.com/developers/apps\n")
		fmt.Fprintf(os.Stderr, "  2. Note your App Key from the app settings\n")
		fmt.Fprintf(os.Stderr, "  3. Add http://localhost:8089/callback to OAuth 2 Redirect URIs\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Using environment variable\n")
		fmt.Fprintf(os.Stderr, "  export DROPBOX_APP_KEY=your_app_key\n")
		fmt.Fprintf(os.Stderr, "  %s dropbox-auth\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Using command-line flag\n")
		fmt.Fprintf(os.Stderr, "  %s dropbox-auth -app-key=your_app_key\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Manual flow (no local server)\n")
		fmt.Fprintf(os.Stderr, "  %s dropbox-auth -manual\n", os.Args[0])
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if cmd.AppKey == "" {
		return fmt.Errorf("dropbox app key required: set DROPBOX_APP_KEY environment variable or use -app-key flag")
	}

	cmd.RedirectURI = fmt.Sprintf("http://localhost:%d/callback", cmd.Port)

	return nil
}

// Run executes the Dropbox OAuth flow
func (cmd *DropboxAuthCommand) Run() error {
	fmt.Println("Dropbox OAuth Flow")
	fmt.Println("==================")

	// Create provider
	provider := providers.NewDropboxProvider(cmd.AppKey)

	// Create token store if saving is enabled
	var store *tokenstore.TokenStore
	if !cmd.NoSave {
		var err error
		store, err = tokenstore.New(tokenstore.Config{
			DatabasePath: cmd.DatabasePath,
		})
		if err != nil {
			return fmt.Errorf("failed to open token store: %w", err)
		}
		defer store.Close()
	}

	// Create flow handler
	handler := oauth2.NewFlowHandler(provider, store)

	ctx := context.Background()

	if cmd.Manual {
		return cmd.runManualFlow(ctx, handler)
	}

	return cmd.runServerFlow(ctx, handler)
}

func (cmd *DropboxAuthCommand) runManualFlow(ctx context.Context, handler *oauth2.FlowHandler) error {
	// Get authorization URL
	authURL, codeVerifier, err := handler.GetManualAuthURL()
	if err != nil {
		return fmt.Errorf("failed to build auth URL: %w", err)
	}

	fmt.Println("\nManual Authorization Flow")
	fmt.Println("-------------------------")
	fmt.Println("\n1. Open this URL in your browser:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println("\n2. Authorize the application")
	fmt.Println("3. Copy the authorization code and paste it below:")
	fmt.Println()

	fmt.Print("Authorization code: ")
	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read authorization code: %w", err)
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("authorization code cannot be empty")
	}

	// Exchange code for tokens
	result, err := handler.CompleteWebFlow(ctx, code, codeVerifier, "", "", "")
	if err != nil {
		return fmt.Errorf("failed to complete flow: %w", err)
	}

	cmd.printResult(result)
	return nil
}

func (cmd *DropboxAuthCommand) runServerFlow(ctx context.Context, handler *oauth2.FlowHandler) error {
	fmt.Printf("\nStarting local server on port %d...\n", cmd.Port)

	cfg := oauth2.CLIFlowConfig{
		Port:    cmd.Port,
		Timeout: 5 * time.Minute,
		OnAuthURL: func(url string) {
			fmt.Println("\nOpen this URL in your browser to authorize:")
			fmt.Println()
			fmt.Println(url)
			fmt.Println("\nWaiting for authorization...")
		},
		OnCodeReceived: func() {
			fmt.Println("\nAuthorization code received!")
		},
		OnTokenReceived: nil, // We'll print manually
		OnError: func(err error) {
			fmt.Printf("\nError: %v\n", err)
		},
	}

	result, err := handler.RunCLIFlow(ctx, cfg)
	if err != nil {
		return err
	}

	cmd.printResult(result)
	return nil
}

func (cmd *DropboxAuthCommand) printResult(result *oauth2.FlowResult) {
	fmt.Println("\nSuccessfully obtained Dropbox tokens!")

	if !cmd.NoSave {
		fmt.Printf("\nTokens saved securely to database: %s\n", cmd.DatabasePath)
		fmt.Printf("   Account ID: %s\n", result.AccountID)
		fmt.Printf("   Encryption key: %s\n", tokenstore.GetKeyFilePath(""))
	}

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("TOKEN INFO:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  Token Type: %s\n", result.TokenType)
	if result.ExpiresAt != nil {
		remaining := time.Until(*result.ExpiresAt)
		fmt.Printf("  Expires In: %.1f hours\n", remaining.Hours())
	}
	if result.Scope != "" {
		fmt.Printf("  Scope: %s\n", result.Scope)
	}
	fmt.Printf("  Account ID: %s\n", result.AccountID)

	// Only print tokens if --no-save
	if cmd.NoSave {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("ACCESS TOKEN (use with -token flag or DROPBOX_ACCESS_TOKEN):")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("\n%s\n", result.AccessToken)

		if result.RefreshToken != "" {
			fmt.Println("\n" + strings.Repeat("-", 60))
			fmt.Println("REFRESH TOKEN (save this for long-term access):")
			fmt.Println(strings.Repeat("-", 60))
			fmt.Printf("\n%s\n", result.RefreshToken)
		}

		fmt.Println("\nUsage:")
		fmt.Println("  export DROPBOX_ACCESS_TOKEN=<access_token>")
		fmt.Printf("  %s moonreader-dropbox\n", os.Args[0])
	} else {
		fmt.Println("\nUsage:")
		fmt.Printf("  %s moonreader-dropbox -db %s\n", os.Args[0], cmd.DatabasePath)
		fmt.Println("\n   (Tokens will be loaded automatically from the database)")
	}
}
