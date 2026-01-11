package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

const (
	dropboxAuthURL  = "https://www.dropbox.com/oauth2/authorize"
	dropboxTokenURL = "https://api.dropboxapi.com/oauth2/token"
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
	fs.StringVar(&cmd.DatabasePath, "db", "./assistant.db", "Path to the database for storing tokens")
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
		return fmt.Errorf("Dropbox App Key required. Set DROPBOX_APP_KEY environment variable or use -app-key flag")
	}

	cmd.RedirectURI = fmt.Sprintf("http://localhost:%d/callback", cmd.Port)

	return nil
}

// Run executes the Dropbox OAuth flow
func (cmd *DropboxAuthCommand) Run() error {
	fmt.Println("🔐 Dropbox OAuth Flow")
	fmt.Println("=====================")

	// Generate PKCE code verifier and challenge
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Generate state for CSRF protection
	state, err := generateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	if cmd.Manual {
		return cmd.runManualFlow(codeVerifier, codeChallenge, state)
	}

	return cmd.runServerFlow(codeVerifier, codeChallenge, state)
}

func (cmd *DropboxAuthCommand) runManualFlow(codeVerifier, codeChallenge, state string) error {
	// Build authorization URL for manual flow (no redirect)
	authURL := cmd.buildAuthURL(codeChallenge, state, "")

	fmt.Println("\n📋 Manual Authorization Flow")
	fmt.Println("----------------------------")
	fmt.Println("\n1. Open this URL in your browser:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println("\n2. Authorize the application")
	fmt.Println("3. Copy the authorization code and paste it below:")
	fmt.Println()

	fmt.Print("Authorization code: ")
	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return fmt.Errorf("failed to read authorization code: %w", err)
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("authorization code cannot be empty")
	}

	return cmd.exchangeCodeForToken(code, codeVerifier, "")
}

func (cmd *DropboxAuthCommand) runServerFlow(codeVerifier, codeChallenge, state string) error {
	// Build authorization URL with redirect
	authURL := cmd.buildAuthURL(codeChallenge, state, cmd.RedirectURI)

	fmt.Printf("\n🌐 Starting local server on port %d...\n", cmd.Port)

	// Channel to receive the authorization code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Create server with handler
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cmd.Port),
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Check for errors
		if errParam := query.Get("error"); errParam != "" {
			errDesc := query.Get("error_description")
			errChan <- fmt.Errorf("authorization error: %s - %s", errParam, errDesc)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body><h1>❌ Authorization Failed</h1><p>%s: %s</p><p>You can close this window.</p></body></html>`, errParam, errDesc)
			return
		}

		// Verify state
		receivedState := query.Get("state")
		if receivedState != state {
			errChan <- fmt.Errorf("state mismatch: possible CSRF attack")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><h1>❌ Security Error</h1><p>State mismatch detected.</p></body></html>`)
			return
		}

		// Get authorization code
		code := query.Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><h1>❌ Error</h1><p>No authorization code received.</p></body></html>`)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h1>✅ Authorization Successful!</h1><p>You can close this window and return to the terminal.</p></body></html>`)
		codeChan <- code
	})

	// Check if port is available
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cmd.Port))
	if err != nil {
		return fmt.Errorf("port %d is not available: %w", cmd.Port, err)
	}

	// Start server in background
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	fmt.Println("\n📋 Open this URL in your browser to authorize:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println("\n⏳ Waiting for authorization...")

	// Wait for code or error with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var code string
	select {
	case code = <-codeChan:
		fmt.Println("\n✅ Authorization code received!")
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return err
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return fmt.Errorf("timeout waiting for authorization (5 minutes)")
	}

	// Shutdown server
	_ = server.Shutdown(context.Background())

	return cmd.exchangeCodeForToken(code, codeVerifier, cmd.RedirectURI)
}

func (cmd *DropboxAuthCommand) buildAuthURL(codeChallenge, state, redirectURI string) string {
	params := url.Values{}
	params.Set("client_id", cmd.AppKey)
	params.Set("response_type", "code")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("token_access_type", "offline") // Get refresh token

	if redirectURI != "" {
		params.Set("redirect_uri", redirectURI)
	}

	return dropboxAuthURL + "?" + params.Encode()
}

func (cmd *DropboxAuthCommand) exchangeCodeForToken(code, codeVerifier, redirectURI string) error {
	fmt.Println("\n🔄 Exchanging authorization code for access token...")

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", cmd.AppKey)
	data.Set("code_verifier", codeVerifier)

	if redirectURI != "" {
		data.Set("redirect_uri", redirectURI)
	}

	req, err := http.NewRequest("POST", dropboxTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil {
			return fmt.Errorf("token exchange failed: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		UID          string `json:"uid"`
		AccountID    string `json:"account_id"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	fmt.Println("\n✅ Successfully obtained Dropbox tokens!")

	// Calculate expiry time
	var expiresAt *time.Time
	if tokenResp.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		expiresAt = &exp
	}

	// Save tokens to database unless --no-save is specified
	if !cmd.NoSave {
		if err := cmd.saveTokens(tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.TokenType, tokenResp.AccountID, tokenResp.Scope, expiresAt); err != nil {
			fmt.Printf("\n⚠️  Warning: Failed to save tokens to database: %v\n", err)
			fmt.Println("   Tokens will be printed below for manual storage.")
		} else {
			fmt.Printf("\n💾 Tokens saved securely to database: %s\n", cmd.DatabasePath)
			fmt.Printf("   Account ID: %s\n", tokenResp.AccountID)
			fmt.Printf("   Encryption key: %s\n", tokenstore.GetKeyFilePath(""))
		}
	}

	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("TOKEN INFO:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  Token Type: %s\n", tokenResp.TokenType)
	if tokenResp.ExpiresIn > 0 {
		fmt.Printf("  Expires In: %d seconds (~%.1f hours)\n", tokenResp.ExpiresIn, float64(tokenResp.ExpiresIn)/3600)
	}
	if tokenResp.Scope != "" {
		fmt.Printf("  Scope: %s\n", tokenResp.Scope)
	}
	fmt.Printf("  Account ID: %s\n", tokenResp.AccountID)

	// Only print tokens if --no-save or if we need to show them
	if cmd.NoSave {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("ACCESS TOKEN (use with -token flag or DROPBOX_ACCESS_TOKEN):")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("\n%s\n", tokenResp.AccessToken)

		if tokenResp.RefreshToken != "" {
			fmt.Println("\n" + strings.Repeat("-", 60))
			fmt.Println("REFRESH TOKEN (save this for long-term access):")
			fmt.Println(strings.Repeat("-", 60))
			fmt.Printf("\n%s\n", tokenResp.RefreshToken)
		}

		fmt.Println("\n💡 Usage:")
		fmt.Println("  export DROPBOX_ACCESS_TOKEN=<access_token>")
		fmt.Printf("  %s moonreader-dropbox\n", os.Args[0])
	} else {
		fmt.Println("\n💡 Usage:")
		fmt.Printf("  %s moonreader-dropbox -db %s\n", os.Args[0], cmd.DatabasePath)
		fmt.Println("\n   (Tokens will be loaded automatically from the database)")
	}

	return nil
}

// saveTokens saves the OAuth tokens to the encrypted database
func (cmd *DropboxAuthCommand) saveTokens(accessToken, refreshToken, tokenType, accountID, scope string, expiresAt *time.Time) error {
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: cmd.DatabasePath,
	})
	if err != nil {
		return fmt.Errorf("failed to open token store: %w", err)
	}
	defer store.Close()

	token := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    accountID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
		ExpiresAt:    expiresAt,
		Scope:        scope,
	}

	if err := store.SaveToken(token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	return nil
}

// generateCodeVerifier creates a random code verifier for PKCE
func generateCodeVerifier() (string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// URL-safe base64 encoding without padding
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// generateCodeChallenge creates a code challenge from the verifier using S256
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// generateState creates a random state value for CSRF protection
func generateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
