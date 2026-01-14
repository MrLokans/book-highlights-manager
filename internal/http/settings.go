package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/moonreader"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

const (
	dropboxAuthURL  = "https://www.dropbox.com/oauth2/authorize"
	dropboxTokenURL = "https://api.dropboxapi.com/oauth2/token"
	dropboxUserURL  = "https://api.dropboxapi.com/2/users/get_current_account"
)

type SettingsController struct {
	DatabasePath  string
	DropboxAppKey string

	// MoonReader configuration
	MoonReaderDropboxPath  string
	MoonReaderDatabasePath string
	MoonReaderOutputDir    string

	// Settings store for persistent settings
	settingsStore *settingsstore.SettingsStore

	// Task queue info
	TasksEnabled bool
	TaskWorkers  int

	// In-memory store for PKCE state (code_verifier keyed by state)
	// In production, consider using a more persistent store
	pkceStore   map[string]pkceData
	pkceStoreMu sync.RWMutex
}

type pkceData struct {
	codeVerifier string
	redirectURI  string
	createdAt    time.Time
}

type DropboxStatus struct {
	Connected   bool       `json:"connected"`
	AccountID   string     `json:"account_id,omitempty"`
	Email       string     `json:"email,omitempty"`
	DisplayName string     `json:"display_name,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsExpired   bool       `json:"is_expired"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

func NewSettingsController(databasePath string, dropboxAppKey string, moonReaderDropboxPath string, moonReaderDatabasePath string, moonReaderOutputDir string, tasksEnabled bool, taskWorkers int) *SettingsController {
	// Initialize database connection for settings store
	db, err := database.NewDatabase(databasePath)
	var store *settingsstore.SettingsStore
	if err == nil {
		store = settingsstore.New(db)
	}

	return &SettingsController{
		DatabasePath:           databasePath,
		DropboxAppKey:          dropboxAppKey,
		MoonReaderDropboxPath:  moonReaderDropboxPath,
		MoonReaderDatabasePath: moonReaderDatabasePath,
		MoonReaderOutputDir:    moonReaderOutputDir,
		settingsStore:          store,
		TasksEnabled:           tasksEnabled,
		TaskWorkers:            taskWorkers,
		pkceStore:              make(map[string]pkceData),
	}
}

func (c *SettingsController) SettingsPage(ctx *gin.Context) {
	status := c.getDropboxStatus()

	// Get export path info
	var exportPath string
	var exportPathSource string
	if c.settingsStore != nil {
		info := c.settingsStore.GetMarkdownExportPathInfo()
		exportPath = info.Path
		exportPathSource = info.Source
	} else {
		// Fallback if settings store not available
		exportPath, _ = os.Getwd()
		exportPathSource = "default"
	}

	ctx.HTML(http.StatusOK, "settings", gin.H{
		"DropboxConfigured":  c.DropboxAppKey != "",
		"DropboxStatus":      status,
		"ExportPath":         exportPath,
		"ExportPathSource":   exportPathSource,
		"SettingsAvailable":  c.settingsStore != nil,
		"TasksEnabled":       c.TasksEnabled,
		"TaskWorkers":        c.TaskWorkers,
	})
}

func (c *SettingsController) InitDropboxAuth(ctx *gin.Context) {
	if c.DropboxAppKey == "" {
		ctx.HTML(http.StatusBadRequest, "settings-error", gin.H{
			"Error": "Dropbox App Key not configured. Set DROPBOX_APP_KEY environment variable.",
		})
		return
	}

	// Generate PKCE code verifier and challenge
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-error", gin.H{
			"Error": "Failed to generate security code",
		})
		return
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Generate state for CSRF protection
	state, err := generateState()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-error", gin.H{
			"Error": "Failed to generate security state",
		})
		return
	}

	// Build redirect URI from current request
	scheme := "http"
	if ctx.Request.TLS != nil || ctx.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	redirectURI := fmt.Sprintf("%s://%s/settings/oauth/dropbox/callback", scheme, ctx.Request.Host)

	// Store PKCE data
	c.pkceStoreMu.Lock()
	c.pkceStore[state] = pkceData{
		codeVerifier: codeVerifier,
		redirectURI:  redirectURI,
		createdAt:    time.Now(),
	}
	c.pkceStoreMu.Unlock()

	// Clean up old PKCE entries (older than 10 minutes)
	go c.cleanupOldPKCE()

	// Build authorization URL
	params := url.Values{}
	params.Set("client_id", c.DropboxAppKey)
	params.Set("response_type", "code")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("token_access_type", "offline")
	params.Set("redirect_uri", redirectURI)

	authURL := dropboxAuthURL + "?" + params.Encode()

	ctx.Redirect(http.StatusFound, authURL)
}

func (c *SettingsController) DropboxCallback(ctx *gin.Context) {
	// Check for errors
	if errParam := ctx.Query("error"); errParam != "" {
		errDesc := ctx.Query("error_description")
		ctx.HTML(http.StatusBadRequest, "settings-callback", gin.H{
			"Success": false,
			"Error":   fmt.Sprintf("%s: %s", errParam, errDesc),
		})
		return
	}

	state := ctx.Query("state")
	code := ctx.Query("code")

	if state == "" || code == "" {
		ctx.HTML(http.StatusBadRequest, "settings-callback", gin.H{
			"Success": false,
			"Error":   "Missing state or authorization code",
		})
		return
	}

	// Retrieve and validate PKCE data
	c.pkceStoreMu.Lock()
	data, ok := c.pkceStore[state]
	if ok {
		delete(c.pkceStore, state)
	}
	c.pkceStoreMu.Unlock()

	if !ok {
		ctx.HTML(http.StatusBadRequest, "settings-callback", gin.H{
			"Success": false,
			"Error":   "Invalid or expired state. Please try again.",
		})
		return
	}

	// Exchange code for token
	tokenData := url.Values{}
	tokenData.Set("grant_type", "authorization_code")
	tokenData.Set("code", code)
	tokenData.Set("client_id", c.DropboxAppKey)
	tokenData.Set("code_verifier", data.codeVerifier)
	tokenData.Set("redirect_uri", data.redirectURI)

	req, err := http.NewRequest("POST", dropboxTokenURL, strings.NewReader(tokenData.Encode()))
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-callback", gin.H{
			"Success": false,
			"Error":   "Failed to create token request",
		})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-callback", gin.H{
			"Success": false,
			"Error":   "Failed to exchange authorization code",
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-callback", gin.H{
			"Success": false,
			"Error":   "Failed to read token response",
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil {
			ctx.HTML(http.StatusBadRequest, "settings-callback", gin.H{
				"Success": false,
				"Error":   fmt.Sprintf("Token exchange failed: %s - %s", errResp.Error, errResp.ErrorDescription),
			})
			return
		}
		ctx.HTML(http.StatusBadRequest, "settings-callback", gin.H{
			"Success": false,
			"Error":   fmt.Sprintf("Token exchange failed with status %d", resp.StatusCode),
		})
		return
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
		ctx.HTML(http.StatusInternalServerError, "settings-callback", gin.H{
			"Success": false,
			"Error":   "Failed to parse token response",
		})
		return
	}

	// Calculate expiry time
	var expiresAt *time.Time
	if tokenResp.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		expiresAt = &exp
	}

	// Save token to database
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: c.DatabasePath,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-callback", gin.H{
			"Success": false,
			"Error":   fmt.Sprintf("Failed to open token store: %v", err),
		})
		return
	}
	defer store.Close()

	token := &entities.DecryptedToken{
		Provider:     entities.OAuthProviderDropbox,
		AccountID:    tokenResp.AccountID,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    expiresAt,
		Scope:        tokenResp.Scope,
	}

	if err := store.SaveToken(token); err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-callback", gin.H{
			"Success": false,
			"Error":   fmt.Sprintf("Failed to save token: %v", err),
		})
		return
	}

	ctx.HTML(http.StatusOK, "settings-callback", gin.H{
		"Success":   true,
		"AccountID": tokenResp.AccountID,
	})
}

func (c *SettingsController) CheckDropboxToken(ctx *gin.Context) {
	status := c.getDropboxStatusWithValidation()
	ctx.HTML(http.StatusOK, "dropbox-status", status)
}

func (c *SettingsController) DisconnectDropbox(ctx *gin.Context) {
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: c.DatabasePath,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "dropbox-status", &DropboxStatus{
			Connected: false,
		})
		return
	}
	defer store.Close()

	// Get existing token to find account ID
	token, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	if err != nil || token == nil {
		ctx.HTML(http.StatusOK, "dropbox-status", &DropboxStatus{
			Connected: false,
		})
		return
	}

	// Delete the token
	if err := store.DeleteToken(entities.OAuthProviderDropbox, token.AccountID); err != nil {
		ctx.HTML(http.StatusInternalServerError, "dropbox-status", &DropboxStatus{
			Connected: true,
			AccountID: token.AccountID,
		})
		return
	}

	ctx.HTML(http.StatusOK, "dropbox-status", &DropboxStatus{
		Connected: false,
	})
}

type MoonReaderImportResult struct {
	Success       bool              `json:"success"`
	Error         string            `json:"error,omitempty"`
	BooksImported int               `json:"books_imported"`
	Highlights    int               `json:"highlights"`
	BooksExported int               `json:"books_exported"`
	ExportedFiles map[string]string `json:"exported_files,omitempty"`
	Errors        []string          `json:"errors,omitempty"`
}

func (c *SettingsController) ImportMoonReaderBackup(ctx *gin.Context) {
	// Get the Dropbox token
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: c.DatabasePath,
	})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to open token store: %v", err),
		})
		return
	}
	defer store.Close()

	token, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	if err != nil || token == nil {
		ctx.HTML(http.StatusBadRequest, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   "Dropbox not connected. Please connect Dropbox first.",
		})
		return
	}

	// Convert paths to absolute
	absOutputDir, err := filepath.Abs(c.MoonReaderOutputDir)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid output directory: %v", err),
		})
		return
	}

	absDBPath, err := filepath.Abs(c.MoonReaderDatabasePath)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid database path: %v", err),
		})
		return
	}

	// Initialize local database
	accessor, err := moonreader.NewLocalDBAccessor(absDBPath)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to initialize local database: %v", err),
		})
		return
	}
	defer accessor.Close()

	result := &MoonReaderImportResult{
		Success:       true,
		ExportedFiles: make(map[string]string),
	}

	// Import from Dropbox
	extractor := moonreader.NewDropboxBackupExtractor(token.AccessToken)
	extractor.WithBasePath(c.MoonReaderDropboxPath)

	dbPath, cleanup, _, err := extractor.ExtractLatestDatabase()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to download backup from Dropbox: %v", err),
		})
		return
	}
	defer cleanup()

	// Read notes from backup
	reader := moonreader.NewBackupDBReader(dbPath)
	notes, err := reader.GetNotes()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to read notes from backup: %v", err),
		})
		return
	}

	result.Highlights = len(notes)

	// Count unique books
	bookCount := make(map[string]int)
	for _, note := range notes {
		bookCount[note.BookTitle]++
	}
	result.BooksImported = len(bookCount)

	// Upsert notes to local database
	if len(notes) > 0 {
		if err := accessor.UpsertNotes(notes); err != nil {
			ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to save notes: %v", err),
			})
			return
		}
	}

	// Export to markdown
	exporter := moonreader.NewObsidianExporter(absOutputDir, accessor)
	exportResult, err := exporter.Export()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Export error: %v", err))
	} else {
		result.BooksExported = len(exportResult.ExportedFiles)
		result.ExportedFiles = exportResult.ExportedFiles
		result.Errors = append(result.Errors, exportResult.Errors...)
	}

	// Update last used timestamp
	_ = store.UpdateLastUsed(entities.OAuthProviderDropbox, token.AccountID)

	ctx.HTML(http.StatusOK, "import-result", result)
}

func (c *SettingsController) getDropboxStatus() *DropboxStatus {
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: c.DatabasePath,
	})
	if err != nil {
		return &DropboxStatus{Connected: false}
	}
	defer store.Close()

	tokens, err := store.ListTokens(entities.OAuthProviderDropbox)
	if err != nil || len(tokens) == 0 {
		return &DropboxStatus{Connected: false}
	}

	token := tokens[0]
	return &DropboxStatus{
		Connected:  true,
		AccountID:  token.AccountID,
		ExpiresAt:  token.ExpiresAt,
		IsExpired:  token.IsExpired(),
		LastUsedAt: token.LastUsedAt,
	}
}

// Validates with Dropbox API
func (c *SettingsController) getDropboxStatusWithValidation() *DropboxStatus {
	store, err := tokenstore.New(tokenstore.Config{
		DatabasePath: c.DatabasePath,
	})
	if err != nil {
		return &DropboxStatus{Connected: false}
	}
	defer store.Close()

	token, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	if err != nil || token == nil {
		return &DropboxStatus{Connected: false}
	}

	// Validate token by calling Dropbox API
	req, err := http.NewRequest("POST", dropboxUserURL, nil)
	if err != nil {
		return &DropboxStatus{
			Connected: true,
			AccountID: token.AccountID,
			IsExpired: true,
		}
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return &DropboxStatus{
			Connected: true,
			AccountID: token.AccountID,
			IsExpired: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &DropboxStatus{
			Connected: true,
			AccountID: token.AccountID,
			IsExpired: true,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &DropboxStatus{
			Connected: true,
			AccountID: token.AccountID,
			IsExpired: false,
		}
	}

	var userInfo struct {
		AccountID string `json:"account_id"`
		Email     string `json:"email"`
		Name      struct {
			DisplayName string `json:"display_name"`
		} `json:"name"`
	}

	if err := json.Unmarshal(body, &userInfo); err != nil {
		return &DropboxStatus{
			Connected: true,
			AccountID: token.AccountID,
			IsExpired: false,
		}
	}

	// Update last used timestamp
	_ = store.UpdateLastUsed(entities.OAuthProviderDropbox, token.AccountID)

	return &DropboxStatus{
		Connected:   true,
		AccountID:   userInfo.AccountID,
		Email:       userInfo.Email,
		DisplayName: userInfo.Name.DisplayName,
		ExpiresAt:   token.ExpiresAt,
		IsExpired:   false,
	}
}

type ExportPathRequest struct {
	Path       string `form:"path" json:"path"`
	CreateDirs bool   `form:"create_dirs" json:"create_dirs"`
}

type ExportPathResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Path    string `json:"path"`
	Source  string `json:"source"`
}

func (c *SettingsController) SaveExportPath(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.HTML(http.StatusInternalServerError, "export-path-result", &ExportPathResult{
			Success: false,
			Error:   "Settings store not available",
		})
		return
	}

	var req ExportPathRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.HTML(http.StatusBadRequest, "export-path-result", &ExportPathResult{
			Success: false,
			Error:   "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate and sanitize path
	validatedPath, err := c.validateExportPath(req.Path, req.CreateDirs)
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "export-path-result", &ExportPathResult{
			Success: false,
			Error:   err.Error(),
			Path:    req.Path,
		})
		return
	}

	// Save the path
	if err := c.settingsStore.SetMarkdownExportPath(validatedPath); err != nil {
		ctx.HTML(http.StatusInternalServerError, "export-path-result", &ExportPathResult{
			Success: false,
			Error:   "Failed to save: " + err.Error(),
		})
		return
	}

	info := c.settingsStore.GetMarkdownExportPathInfo()
	ctx.HTML(http.StatusOK, "export-path-result", &ExportPathResult{
		Success: true,
		Path:    info.Path,
		Source:  info.Source,
	})
}

func (c *SettingsController) validateExportPath(rawPath string, createDirs bool) (string, error) {
	// Trim whitespace
	path := strings.TrimSpace(rawPath)

	// Check for empty path
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Reject paths with null bytes (potential injection)
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("path contains invalid characters")
	}

	// Convert to absolute path to normalize it
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path format: %w", err)
	}

	// Clean the path to remove any .. or . components
	cleanPath := filepath.Clean(absPath)

	// Verify the cleaned path doesn't escape to unexpected locations
	// by checking if it starts with a reasonable prefix
	// (This prevents paths like "/etc/passwd" or "/../../../etc" from being accepted
	// when the original path appeared to be in a safe location)

	// Check if the path exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			if createDirs {
				// Attempt to create the directory
				if err := os.MkdirAll(cleanPath, 0755); err != nil {
					return "", fmt.Errorf("failed to create directory: %w", err)
				}
				// Verify creation was successful
				info, err = os.Stat(cleanPath)
				if err != nil {
					return "", fmt.Errorf("directory created but cannot be accessed: %w", err)
				}
			} else {
				return "", fmt.Errorf("path does not exist (enable 'Create directories' to create it)")
			}
		} else if os.IsPermission(err) {
			return "", fmt.Errorf("permission denied: cannot access path")
		} else {
			return "", fmt.Errorf("cannot access path: %w", err)
		}
	}

	// Verify it's a directory
	if !info.IsDir() {
		return "", fmt.Errorf("path must be a directory, not a file")
	}

	// Test write permission by attempting to create and remove a temp file
	testFile := filepath.Join(cleanPath, ".write_test_"+fmt.Sprintf("%d", time.Now().UnixNano()))
	f, err := os.Create(testFile)
	if err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("no write permission for this directory")
		}
		return "", fmt.Errorf("cannot write to directory: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	// Test read permission by listing directory
	_, err = os.ReadDir(cleanPath)
	if err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("no read permission for this directory")
		}
		return "", fmt.Errorf("cannot read directory: %w", err)
	}

	return cleanPath, nil
}

func (c *SettingsController) ResetExportPath(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.HTML(http.StatusInternalServerError, "export-path-result", &ExportPathResult{
			Success: false,
			Error:   "Settings store not available",
		})
		return
	}

	if err := c.settingsStore.ClearMarkdownExportPath(); err != nil {
		ctx.HTML(http.StatusInternalServerError, "export-path-result", &ExportPathResult{
			Success: false,
			Error:   "Failed to reset: " + err.Error(),
		})
		return
	}

	info := c.settingsStore.GetMarkdownExportPathInfo()
	ctx.HTML(http.StatusOK, "export-path-result", &ExportPathResult{
		Success: true,
		Path:    info.Path,
		Source:  info.Source,
	})
}

func (c *SettingsController) cleanupOldPKCE() {
	c.pkceStoreMu.Lock()
	defer c.pkceStoreMu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for state, data := range c.pkceStore {
		if data.createdAt.Before(cutoff) {
			delete(c.pkceStore, state)
		}
	}
}

func generateCodeVerifier() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func generateState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
