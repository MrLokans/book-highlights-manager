package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/mrlokans/assistant/internal/moonreader"
	"github.com/mrlokans/assistant/internal/oauth2"
	"github.com/mrlokans/assistant/internal/oauth2/providers"
	"github.com/mrlokans/assistant/internal/settingsstore"
	"github.com/mrlokans/assistant/internal/storage/providers/dropbox"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

const pkceCleanupInterval = 10 * time.Minute

// SettingsController handles the settings UI and Dropbox OAuth integration.
type SettingsController struct {
	DropboxAppKey string

	MoonReaderDropboxPath string

	exporter     exporters.BookExporter
	auditService *audit.Service

	// Settings store for persistent settings
	settingsStore *settingsstore.SettingsStore

	// Token store for OAuth token management
	tokenStore *tokenstore.TokenStore

	// Shared Dropbox OAuth implementation (same as the dropbox-auth CLI)
	dropbox     *providers.DropboxProvider
	dropboxFlow *oauth2.FlowHandler

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

// DropboxStatus holds the current Dropbox connection state for the UI.
type DropboxStatus struct {
	Connected   bool       `json:"connected"`
	AccountID   string     `json:"account_id,omitempty"`
	Email       string     `json:"email,omitempty"`
	DisplayName string     `json:"display_name,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsExpired   bool       `json:"is_expired"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

// NewSettingsController creates a settings controller with the given dependencies.
func NewSettingsController(settingsStore *settingsstore.SettingsStore, tokenStore *tokenstore.TokenStore, dropboxAppKey string, moonReaderDropboxPath string, exporter exporters.BookExporter, auditService *audit.Service, tasksEnabled bool, taskWorkers int) *SettingsController {
	dropboxProvider := providers.NewDropboxProvider(dropboxAppKey)
	return &SettingsController{
		DropboxAppKey:         dropboxAppKey,
		MoonReaderDropboxPath: moonReaderDropboxPath,
		exporter:              exporter,
		auditService:          auditService,
		settingsStore:         settingsStore,
		tokenStore:            tokenStore,
		TasksEnabled:          tasksEnabled,
		TaskWorkers:           taskWorkers,
		pkceStore:             make(map[string]pkceData),
		dropbox:               dropboxProvider,
		dropboxFlow:           oauth2.NewFlowHandler(dropboxProvider, tokenStore),
	}
}

// SettingsPage renders the main settings page.
func (c *SettingsController) SettingsPage(ctx *gin.Context) {
	status := c.getDropboxStatus()

	RenderPage(ctx, http.StatusOK, "settings", gin.H{
		"ActivePage":        "settings",
		"DropboxConfigured": c.DropboxAppKey != "",
		"DropboxStatus":     status,
		"TasksEnabled":      c.TasksEnabled,
		"TaskWorkers":       c.TaskWorkers,
	})
}

// ImportPage renders the import page: file uploads and connected sources.
func (c *SettingsController) ImportPage(ctx *gin.Context) {
	RenderPage(ctx, http.StatusOK, "import", gin.H{
		"ActivePage":        "import",
		"DropboxConfigured": c.DropboxAppKey != "",
		"DropboxStatus":     c.getDropboxStatus(),
	})
}

// InitDropboxAuth starts the Dropbox OAuth2 authorization flow.
func (c *SettingsController) InitDropboxAuth(ctx *gin.Context) {
	if c.DropboxAppKey == "" {
		ctx.HTML(http.StatusBadRequest, "settings-error", gin.H{
			"Error": "Dropbox App Key not configured. Set DROPBOX_APP_KEY environment variable.",
		})
		return
	}

	// Build redirect URI from current request, respecting reverse proxy headers
	scheme := "http"
	if ctx.Request.TLS != nil || ctx.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := getEffectiveHost(ctx)
	redirectURI := fmt.Sprintf("%s://%s/settings/oauth/dropbox/callback", scheme, host)

	authURL, state, codeVerifier, err := c.dropboxFlow.StartWebFlow(oauth2.WebFlowConfig{RedirectURL: redirectURI})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-error", gin.H{
			"Error": "Failed to start Dropbox authorization",
		})
		return
	}

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

	ctx.Redirect(http.StatusFound, authURL)
}

// DropboxCallback handles the OAuth2 callback from Dropbox.
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

	if c.tokenStore == nil {
		ctx.HTML(http.StatusInternalServerError, "settings-callback", gin.H{
			"Success": false,
			"Error":   "Token store not configured",
		})
		return
	}

	// State was already matched via the PKCE store lookup above.
	result, err := c.dropboxFlow.CompleteWebFlow(ctx.Request.Context(), code, data.codeVerifier, data.redirectURI, "", "")
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "settings-callback", gin.H{
			"Success": false,
			"Error":   err.Error(),
		})
		return
	}

	ctx.HTML(http.StatusOK, "settings-callback", gin.H{
		"Success":   true,
		"AccountID": result.AccountID,
	})
}

// CheckDropboxToken verifies whether a valid Dropbox token exists.
func (c *SettingsController) CheckDropboxToken(ctx *gin.Context) {
	status := c.getDropboxStatusWithValidation(ctx.Request.Context())
	ctx.HTML(http.StatusOK, "dropbox-status", status)
}

// DisconnectDropbox removes the stored Dropbox OAuth token.
func (c *SettingsController) DisconnectDropbox(ctx *gin.Context) {
	store := c.tokenStore
	if store == nil {
		ctx.HTML(http.StatusInternalServerError, "dropbox-status", &DropboxStatus{
			Connected: false,
		})
		return
	}

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

// MoonReaderImportResult summarises a MoonReader backup import.
type MoonReaderImportResult struct {
	Success       bool     `json:"success"`
	Error         string   `json:"error,omitempty"`
	BooksImported int      `json:"books_imported"`
	Highlights    int      `json:"highlights"`
	Added         int      `json:"highlights_added"`
	BooksExported int      `json:"books_exported"`
	Errors        []string `json:"errors,omitempty"`
}

// ImportMoonReaderBackup imports the latest Moon+ Reader backup from Dropbox into the library.
func (c *SettingsController) ImportMoonReaderBackup(ctx *gin.Context) {
	store := c.tokenStore
	if store == nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   "Token store not configured",
		})
		return
	}

	token, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	if err != nil || token == nil {
		ctx.HTML(http.StatusBadRequest, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   "Dropbox not connected. Please connect Dropbox first.",
		})
		return
	}

	// With an app key the token is refreshed on use if it has expired.
	var tokenSource oauth2.TokenSource = oauth2.NewStaticTokenSource(token.AccessToken, token.AccountID)
	if c.DropboxAppKey != "" {
		tokenSource = oauth2.NewStoredTokenSource(c.dropbox, store, token.AccountID)
	}
	client := dropbox.NewClient(tokenSource)

	dbPath, _, cleanup, err := moonreader.ExtractLatestDropboxBackup(ctx.Request.Context(), client, c.MoonReaderDropboxPath)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to download backup from Dropbox: %v", err),
		})
		return
	}
	defer cleanup()

	notes, err := moonreader.NewBackupDBReader(dbPath).GetNotes()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to read notes from backup: %v", err),
		})
		return
	}

	highlights := make([]MoonReaderHighlight, len(notes))
	for i, note := range notes {
		highlights[i] = MoonReaderHighlight(*note)
	}
	books := moonReaderHighlightsToBooks(highlights)

	exportResult, err := importBooks(ctx, c.exporter, c.auditService, "moonreader", "Moon+ Reader (Dropbox)", books)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "import-result", &MoonReaderImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to import: %v", err),
		})
		return
	}

	_ = store.UpdateLastUsed(entities.OAuthProviderDropbox, token.AccountID)

	ctx.HTML(http.StatusOK, "import-result", &MoonReaderImportResult{
		Success:       true,
		Highlights:    exportResult.HighlightsProcessed,
		Added:         exportResult.HighlightsAdded,
		BooksImported: exportResult.BooksProcessed,
		BooksExported: exportResult.BooksProcessed,
	})
}

func (c *SettingsController) getDropboxStatus() *DropboxStatus {
	store := c.tokenStore
	if store == nil {
		return &DropboxStatus{Connected: false}
	}

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
func (c *SettingsController) getDropboxStatusWithValidation(ctx context.Context) *DropboxStatus {
	store := c.tokenStore
	if store == nil {
		return &DropboxStatus{Connected: false}
	}

	token, err := store.GetTokenByProvider(entities.OAuthProviderDropbox)
	if err != nil || token == nil {
		return &DropboxStatus{Connected: false}
	}

	// Validate token by calling Dropbox API
	account, err := c.dropbox.GetAccount(ctx, token.AccessToken)
	if err != nil {
		return &DropboxStatus{
			Connected: true,
			AccountID: token.AccountID,
			IsExpired: true,
		}
	}

	// Update last used timestamp
	_ = store.UpdateLastUsed(entities.OAuthProviderDropbox, token.AccountID)

	return &DropboxStatus{
		Connected:   true,
		AccountID:   account.AccountID,
		Email:       account.Email,
		DisplayName: account.Name.DisplayName,
		ExpiresAt:   token.ExpiresAt,
		IsExpired:   false,
	}
}

func (c *SettingsController) cleanupOldPKCE() {
	c.pkceStoreMu.Lock()
	defer c.pkceStoreMu.Unlock()

	cutoff := time.Now().Add(-pkceCleanupInterval)
	for state, data := range c.pkceStore {
		if data.createdAt.Before(cutoff) {
			delete(c.pkceStore, state)
		}
	}
}

// getEffectiveHost returns the host that the client sees, considering reverse proxy headers.
func getEffectiveHost(c *gin.Context) string {
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		if first, _, found := strings.Cut(forwardedHost, ","); found {
			forwardedHost = strings.TrimSpace(first)
		}
		return forwardedHost
	}
	return c.Request.Host
}
