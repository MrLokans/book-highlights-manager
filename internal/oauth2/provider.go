package oauth2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

// ProviderConfig contains the configuration needed for OAuth2 authorization
type ProviderConfig struct {
	ClientID     string
	ClientSecret string // Optional for PKCE flows
	AuthURL      string
	TokenURL     string
	Scopes       []string
}

// TokenResponse contains tokens returned from the OAuth2 provider
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int // seconds until expiry
	Scope        string
	AccountID    string // Provider-specific account identifier
}

// ExpiresAt calculates the absolute expiry time from ExpiresIn
func (t *TokenResponse) ExpiresAt() *time.Time {
	if t.ExpiresIn <= 0 {
		return nil
	}
	exp := time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	return &exp
}

// Provider defines the interface for OAuth2 providers
type Provider interface {
	// Name returns the provider identifier (e.g., "dropbox", "google")
	Name() entities.OAuthProvider

	// Config returns the provider's OAuth2 configuration
	Config() ProviderConfig

	// BuildAuthURL constructs the authorization URL for the OAuth2 flow.
	// Returns the auth URL, PKCE code verifier (if applicable), and state parameter.
	BuildAuthURL(redirectURL string) (authURL, codeVerifier, state string, err error)

	// ExchangeCode exchanges an authorization code for tokens
	ExchangeCode(ctx context.Context, code, codeVerifier, redirectURL string) (*TokenResponse, error)

	// RefreshToken exchanges a refresh token for a new access token
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)

	// GetAccountInfo retrieves the account identifier for the authenticated user
	GetAccountInfo(ctx context.Context, accessToken string) (accountID string, err error)
}

// Registry manages registered OAuth2 providers
type Registry struct {
	mu        sync.RWMutex
	providers map[entities.OAuthProvider]Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[entities.OAuthProvider]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get retrieves a provider by name
func (r *Registry) Get(name entities.OAuthProvider) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", name)
	}
	return p, nil
}

// List returns all registered provider names
func (r *Registry) List() []entities.OAuthProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]entities.OAuthProvider, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// All returns all registered providers
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// DefaultRegistry is the global provider registry
var DefaultRegistry = NewRegistry()

// Register adds a provider to the default registry
func Register(p Provider) {
	DefaultRegistry.Register(p)
}

// GetProvider retrieves a provider from the default registry
func GetProvider(name entities.OAuthProvider) (Provider, error) {
	return DefaultRegistry.Get(name)
}
