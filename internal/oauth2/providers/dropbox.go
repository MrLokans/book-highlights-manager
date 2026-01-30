package providers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/oauth2"
)

const (
	dropboxAuthURL  = "https://www.dropbox.com/oauth2/authorize"
	dropboxTokenURL = "https://api.dropboxapi.com/oauth2/token"
	dropboxAPIURL   = "https://api.dropboxapi.com/2"
)

// DropboxProvider implements OAuth2 for Dropbox using PKCE
type DropboxProvider struct {
	appKey     string
	httpClient *http.Client
}

// NewDropboxProvider creates a new Dropbox OAuth2 provider
func NewDropboxProvider(appKey string) *DropboxProvider {
	return &DropboxProvider{
		appKey: appKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *DropboxProvider) Name() entities.OAuthProvider {
	return entities.OAuthProviderDropbox
}

func (p *DropboxProvider) Config() oauth2.ProviderConfig {
	return oauth2.ProviderConfig{
		ClientID: p.appKey,
		AuthURL:  dropboxAuthURL,
		TokenURL: dropboxTokenURL,
		Scopes:   []string{}, // Dropbox doesn't use scope in auth URL, it's set in app settings
	}
}

func (p *DropboxProvider) BuildAuthURL(redirectURL string) (authURL, codeVerifier, state string, err error) {
	// Generate PKCE code verifier and challenge
	codeVerifier, err = generateCodeVerifier()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Generate state for CSRF protection
	state, err = generateState()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	params := url.Values{}
	params.Set("client_id", p.appKey)
	params.Set("response_type", "code")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("token_access_type", "offline") // Get refresh token

	if redirectURL != "" {
		params.Set("redirect_uri", redirectURL)
	}

	authURL = dropboxAuthURL + "?" + params.Encode()
	return authURL, codeVerifier, state, nil
}

func (p *DropboxProvider) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURL string) (*oauth2.TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", p.appKey)
	data.Set("code_verifier", codeVerifier)

	if redirectURL != "" {
		data.Set("redirect_uri", redirectURL)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("token exchange failed: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
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
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &oauth2.TokenResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
		Scope:        tokenResp.Scope,
		AccountID:    tokenResp.AccountID,
	}, nil
}

func (p *DropboxProvider) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", p.appKey)

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("token refresh failed: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Dropbox refresh doesn't return a new refresh token
	return &oauth2.TokenResponse{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		ExpiresIn:   tokenResp.ExpiresIn,
	}, nil
}

func (p *DropboxProvider) GetAccountInfo(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", dropboxAPIURL+"/users/get_current_account", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get account info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get account info (status %d): %s", resp.StatusCode, string(body))
	}

	var accountResp struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&accountResp); err != nil {
		return "", fmt.Errorf("failed to parse account response: %w", err)
	}

	return accountResp.AccountID, nil
}

// generateCodeVerifier creates a random code verifier for PKCE
func generateCodeVerifier() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
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

// RegisterDropbox registers the Dropbox provider with the given app key
func RegisterDropbox(appKey string) {
	if appKey == "" {
		return
	}
	oauth2.Register(NewDropboxProvider(appKey))
}
