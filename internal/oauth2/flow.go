package oauth2

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

// FlowResult contains the result of a completed OAuth2 flow
type FlowResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    *time.Time
	AccountID    string
	Scope        string
}

// FlowHandler handles OAuth2 authorization flows
type FlowHandler struct {
	provider   Provider
	tokenStore *tokenstore.TokenStore
}

// NewFlowHandler creates a new OAuth2 flow handler
func NewFlowHandler(provider Provider, store *tokenstore.TokenStore) *FlowHandler {
	return &FlowHandler{
		provider:   provider,
		tokenStore: store,
	}
}

// CLIFlowConfig configures a CLI-based OAuth2 flow
type CLIFlowConfig struct {
	Port            int                        // Local server port for callback (default: 8089)
	Timeout         time.Duration              // Timeout waiting for authorization (default: 5 minutes)
	OnAuthURL       func(url string)           // Called with the authorization URL to display
	OnCodeReceived  func()                     // Called when authorization code is received
	OnTokenReceived func(result *FlowResult)   // Called when tokens are received
	OnError         func(err error)            // Called on error
}

// DefaultCLIFlowConfig returns default configuration for CLI flow
func DefaultCLIFlowConfig() CLIFlowConfig {
	return CLIFlowConfig{
		Port:    8089,
		Timeout: 5 * time.Minute,
		OnAuthURL: func(url string) {
			fmt.Println("\nOpen this URL in your browser to authorize:")
			fmt.Println()
			fmt.Println(url)
		},
		OnCodeReceived: func() {
			fmt.Println("\nAuthorization code received!")
		},
		OnTokenReceived: func(result *FlowResult) {
			fmt.Printf("\nSuccessfully authenticated account: %s\n", result.AccountID)
		},
		OnError: func(err error) {
			fmt.Printf("\nError: %v\n", err)
		},
	}
}

// RunCLIFlow executes the OAuth2 flow with a local callback server
func (h *FlowHandler) RunCLIFlow(ctx context.Context, cfg CLIFlowConfig) (*FlowResult, error) {
	redirectURL := fmt.Sprintf("http://localhost:%d/callback", cfg.Port)

	// Build authorization URL
	authURL, codeVerifier, state, err := h.provider.BuildAuthURL(redirectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth URL: %w", err)
	}

	// Display the URL
	if cfg.OnAuthURL != nil {
		cfg.OnAuthURL(authURL)
	}

	// Start local server to receive callback
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Check for errors
		if errParam := query.Get("error"); errParam != "" {
			errDesc := query.Get("error_description")
			errChan <- fmt.Errorf("authorization error: %s - %s", errParam, errDesc)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body><h1>Authorization Failed</h1><p>%s: %s</p><p>You can close this window.</p></body></html>`, errParam, errDesc)
			return
		}

		// Verify state
		receivedState := query.Get("state")
		if receivedState != state {
			errChan <- fmt.Errorf("state mismatch: possible CSRF attack")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><h1>Security Error</h1><p>State mismatch detected.</p></body></html>`)
			return
		}

		// Get authorization code
		code := query.Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><h1>Error</h1><p>No authorization code received.</p></body></html>`)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h1>Authorization Successful!</h1><p>You can close this window and return to the terminal.</p></body></html>`)
		codeChan <- code
	})

	// Check if port is available
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("port %d is not available: %w", cfg.Port, err)
	}

	// Start server in background
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for code or error with timeout
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var code string
	select {
	case code = <-codeChan:
		if cfg.OnCodeReceived != nil {
			cfg.OnCodeReceived()
		}
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return nil, err
	case <-timeoutCtx.Done():
		_ = server.Shutdown(context.Background())
		return nil, fmt.Errorf("timeout waiting for authorization")
	}

	// Shutdown server
	_ = server.Shutdown(context.Background())

	// Exchange code for tokens
	return h.exchangeAndSave(ctx, code, codeVerifier, redirectURL, cfg.OnTokenReceived)
}

// ManualFlowConfig configures a manual OAuth2 flow
type ManualFlowConfig struct {
	OnAuthURL       func(url string)         // Called with the authorization URL
	OnTokenReceived func(result *FlowResult) // Called when tokens are received
}

// RunManualFlow executes the OAuth2 flow with manual code entry
func (h *FlowHandler) RunManualFlow(ctx context.Context, code string, cfg ManualFlowConfig) (*FlowResult, error) {
	// Build authorization URL without redirect (for display purposes)
	_, codeVerifier, _, err := h.provider.BuildAuthURL("")
	if err != nil {
		return nil, fmt.Errorf("failed to build auth URL: %w", err)
	}

	// Exchange code for tokens
	return h.exchangeAndSave(ctx, code, codeVerifier, "", cfg.OnTokenReceived)
}

// GetManualAuthURL returns the authorization URL for manual flow
func (h *FlowHandler) GetManualAuthURL() (authURL, codeVerifier string, err error) {
	authURL, codeVerifier, _, err = h.provider.BuildAuthURL("")
	return
}

func (h *FlowHandler) exchangeAndSave(
	ctx context.Context,
	code, codeVerifier, redirectURL string,
	onToken func(*FlowResult),
) (*FlowResult, error) {
	// Exchange code for tokens
	tokenResp, err := h.provider.ExchangeCode(ctx, code, codeVerifier, redirectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Get account info if not provided
	accountID := tokenResp.AccountID
	if accountID == "" {
		accountID, err = h.provider.GetAccountInfo(ctx, tokenResp.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to get account info: %w", err)
		}
	}

	result := &FlowResult{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    tokenResp.ExpiresAt(),
		AccountID:    accountID,
		Scope:        tokenResp.Scope,
	}

	// Save tokens to store
	if h.tokenStore != nil {
		token := &entities.DecryptedToken{
			Provider:     h.provider.Name(),
			AccountID:    accountID,
			AccessToken:  tokenResp.AccessToken,
			RefreshToken: tokenResp.RefreshToken,
			TokenType:    tokenResp.TokenType,
			ExpiresAt:    result.ExpiresAt,
			Scope:        tokenResp.Scope,
		}

		if err := h.tokenStore.SaveToken(token); err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
	}

	if onToken != nil {
		onToken(result)
	}

	return result, nil
}

// WebFlowConfig configures web-based OAuth2 flows
type WebFlowConfig struct {
	RedirectURL string
	State       string // If empty, a random state will be generated
}

// StartWebFlow initiates a web-based OAuth2 flow
// Returns the authorization URL and state for later verification
func (h *FlowHandler) StartWebFlow(cfg WebFlowConfig) (authURL, state, codeVerifier string, err error) {
	authURL, codeVerifier, state, err = h.provider.BuildAuthURL(cfg.RedirectURL)
	if cfg.State != "" {
		// Replace generated state with provided state
		// Note: The provider's BuildAuthURL already includes state in the URL,
		// so this would require rebuilding or string replacement
		// For simplicity, we document that custom state requires URL manipulation
		state = cfg.State
	}
	return
}

// CompleteWebFlow completes a web-based OAuth2 flow after receiving the callback
func (h *FlowHandler) CompleteWebFlow(
	ctx context.Context,
	code, codeVerifier, redirectURL, expectedState, receivedState string,
) (*FlowResult, error) {
	// Verify state
	if expectedState != "" && receivedState != expectedState {
		return nil, fmt.Errorf("state mismatch: expected %q, got %q", expectedState, receivedState)
	}

	return h.exchangeAndSave(ctx, code, codeVerifier, redirectURL, nil)
}
