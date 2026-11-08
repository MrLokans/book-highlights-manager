package http

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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

// The pending OAuth flow (state, PKCE verifier, redirect URI) lives in a short-lived
// HttpOnly cookie scoped to the callback, so it survives restarts and is bound to the
// browser that started it.
const (
	dropboxFlowCookie = "dropbox_oauth"
	dropboxCallback   = "/settings/oauth/dropbox/callback"
	dropboxFlowMaxAge = 10 * time.Minute
)

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
	if isHTTPS(ctx) {
		scheme = "https"
	}
	host := getEffectiveHost(ctx)
	redirectURI := fmt.Sprintf("%s://%s%s", scheme, host, dropboxCallback)

	authURL, state, codeVerifier, err := c.dropboxFlow.StartWebFlow(oauth2.WebFlowConfig{RedirectURL: redirectURI})
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "settings-error", gin.H{
			"Error": "Failed to start Dropbox authorization",
		})
		return
	}

	flow := url.Values{"state": {state}, "verifier": {codeVerifier}, "redirect": {redirectURI}}
	setDropboxFlowCookie(ctx, base64.RawURLEncoding.EncodeToString([]byte(flow.Encode())), int(dropboxFlowMaxAge.Seconds()))

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

	// Single use: clear the pending flow whatever happens next.
	setDropboxFlowCookie(ctx, "", -1)
	flow, ok := readDropboxFlow(ctx)
	if !ok || subtle.ConstantTimeCompare([]byte(flow.Get("state")), []byte(state)) != 1 {
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

	// State was already matched against the flow cookie above.
	result, err := c.dropboxFlow.CompleteWebFlow(ctx.Request.Context(), code, flow.Get("verifier"), flow.Get("redirect"), "", "")
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

	// Revoke at Dropbox first so the refresh token stops working too. Best effort:
	// a network failure must not keep the user from disconnecting locally.
	revokeCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()
	accessToken, err := oauth2.NewStoredTokenSource(c.dropbox, store, token.AccountID).Token(revokeCtx)
	if err == nil {
		err = c.dropbox.RevokeToken(revokeCtx, accessToken)
	}
	if err != nil {
		slog.Warn("Dropbox token revoke failed; deleting local token anyway", "error", err)
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

// setDropboxFlowCookie writes (maxAge > 0) or clears (maxAge < 0) the pending-flow cookie.
func setDropboxFlowCookie(ctx *gin.Context, value string, maxAge int) {
	//nolint:gosec // G124: Secure follows the request scheme so plain-http local dev keeps working
	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     dropboxFlowCookie,
		Value:    value,
		Path:     dropboxCallback,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isHTTPS(ctx),
		SameSite: http.SameSiteLaxMode, // must survive the cross-site redirect back from Dropbox
	})
}

// isHTTPS reports whether the client reached us over HTTPS, directly or via a TLS-terminating proxy.
func isHTTPS(ctx *gin.Context) bool {
	return ctx.Request.TLS != nil || ctx.GetHeader("X-Forwarded-Proto") == "https"
}

// readDropboxFlow decodes the pending-flow cookie set by InitDropboxAuth.
func readDropboxFlow(ctx *gin.Context) (url.Values, bool) {
	raw, err := ctx.Cookie(dropboxFlowCookie)
	if err != nil {
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, false
	}
	flow, err := url.ParseQuery(string(decoded))
	if err != nil || flow.Get("state") == "" || flow.Get("verifier") == "" {
		return nil, false
	}
	return flow, true
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
