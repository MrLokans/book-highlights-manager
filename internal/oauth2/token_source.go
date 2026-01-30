package oauth2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

// TokenSource provides access tokens with automatic refresh capability
type TokenSource interface {
	// Token returns a valid access token, refreshing if necessary
	Token(ctx context.Context) (string, error)

	// ForceRefresh forces a token refresh regardless of expiry status
	ForceRefresh(ctx context.Context) error

	// IsValid returns true if the current token is valid and not expired
	IsValid() bool

	// ExpiresAt returns the token expiry time, or nil if unknown/no expiry
	ExpiresAt() *time.Time

	// AccountID returns the account identifier associated with this token
	AccountID() string
}

// StoredTokenSource provides tokens from the token store with automatic refresh
type StoredTokenSource struct {
	mu sync.RWMutex

	provider   Provider
	tokenStore *tokenstore.TokenStore
	accountID  string

	// Cached token data
	accessToken string
	expiresAt   *time.Time

	// Margin before expiry to trigger refresh (default: 5 minutes)
	refreshMargin time.Duration
}

// StoredTokenSourceOption configures a StoredTokenSource
type StoredTokenSourceOption func(*StoredTokenSource)

// WithRefreshMargin sets the time before expiry to trigger automatic refresh
func WithRefreshMargin(d time.Duration) StoredTokenSourceOption {
	return func(s *StoredTokenSource) {
		s.refreshMargin = d
	}
}

// NewStoredTokenSource creates a TokenSource that retrieves and refreshes tokens from the store
func NewStoredTokenSource(
	provider Provider,
	store *tokenstore.TokenStore,
	accountID string,
	opts ...StoredTokenSourceOption,
) *StoredTokenSource {
	ts := &StoredTokenSource{
		provider:      provider,
		tokenStore:    store,
		accountID:     accountID,
		refreshMargin: 5 * time.Minute,
	}

	for _, opt := range opts {
		opt(ts)
	}

	return ts
}

// Token returns a valid access token, refreshing if necessary
func (s *StoredTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we have a valid cached token, return it
	if s.accessToken != "" && !s.isExpiringSoon() {
		return s.accessToken, nil
	}

	// Load from store
	token, err := s.tokenStore.GetToken(s.provider.Name(), s.accountID)
	if err != nil {
		return "", fmt.Errorf("failed to get token from store: %w", err)
	}
	if token == nil {
		return "", fmt.Errorf("no token found for provider %s, account %s", s.provider.Name(), s.accountID)
	}

	// Update cache
	s.accessToken = token.AccessToken
	s.expiresAt = token.ExpiresAt

	// Check if refresh is needed
	if s.isExpiringSoon() {
		if token.RefreshToken == "" {
			return "", fmt.Errorf("token expired and no refresh token available")
		}

		if err := s.refreshLocked(ctx, token.RefreshToken); err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
	}

	// Update last used timestamp
	_ = s.tokenStore.UpdateLastUsed(s.provider.Name(), s.accountID)

	return s.accessToken, nil
}

// ForceRefresh forces a token refresh
func (s *StoredTokenSource) ForceRefresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, err := s.tokenStore.GetToken(s.provider.Name(), s.accountID)
	if err != nil {
		return fmt.Errorf("failed to get token from store: %w", err)
	}
	if token == nil {
		return fmt.Errorf("no token found for provider %s, account %s", s.provider.Name(), s.accountID)
	}
	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	return s.refreshLocked(ctx, token.RefreshToken)
}

// refreshLocked performs token refresh (caller must hold the lock)
func (s *StoredTokenSource) refreshLocked(ctx context.Context, refreshToken string) error {
	resp, err := s.provider.RefreshToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	// Update store
	newRefreshToken := resp.RefreshToken
	if newRefreshToken == "" {
		newRefreshToken = refreshToken // Keep existing refresh token if not rotated
	}

	expiresAt := resp.ExpiresAt()
	if err := s.tokenStore.UpdateTokenAfterRefresh(
		s.provider.Name(),
		s.accountID,
		resp.AccessToken,
		newRefreshToken,
		expiresAt,
	); err != nil {
		return fmt.Errorf("failed to save refreshed token: %w", err)
	}

	// Update cache
	s.accessToken = resp.AccessToken
	s.expiresAt = expiresAt

	return nil
}

// IsValid returns true if the current token exists and is not expired
func (s *StoredTokenSource) IsValid() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.accessToken == "" {
		return false
	}
	return !s.isExpiringSoon()
}

// ExpiresAt returns the token expiry time
func (s *StoredTokenSource) ExpiresAt() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expiresAt
}

// AccountID returns the associated account identifier
func (s *StoredTokenSource) AccountID() string {
	return s.accountID
}

// isExpiringSoon checks if the token is expired or expiring within the refresh margin
func (s *StoredTokenSource) isExpiringSoon() bool {
	if s.expiresAt == nil {
		return false // No expiry means token doesn't expire
	}
	return time.Now().Add(s.refreshMargin).After(*s.expiresAt)
}

// StaticTokenSource provides a fixed access token without refresh capability
type StaticTokenSource struct {
	accessToken string
	accountID   string
}

// NewStaticTokenSource creates a TokenSource with a fixed token
func NewStaticTokenSource(accessToken, accountID string) *StaticTokenSource {
	return &StaticTokenSource{
		accessToken: accessToken,
		accountID:   accountID,
	}
}

func (s *StaticTokenSource) Token(ctx context.Context) (string, error) {
	return s.accessToken, nil
}

func (s *StaticTokenSource) ForceRefresh(ctx context.Context) error {
	return fmt.Errorf("static token source does not support refresh")
}

func (s *StaticTokenSource) IsValid() bool {
	return s.accessToken != ""
}

func (s *StaticTokenSource) ExpiresAt() *time.Time {
	return nil
}

func (s *StaticTokenSource) AccountID() string {
	return s.accountID
}

// ProviderTokenSource creates a TokenSource from the token store for the most recent account
func ProviderTokenSource(
	provider Provider,
	store *tokenstore.TokenStore,
	opts ...StoredTokenSourceOption,
) (*StoredTokenSource, error) {
	token, err := store.GetTokenByProvider(provider.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	if token == nil {
		return nil, fmt.Errorf("no token found for provider %s", provider.Name())
	}

	return NewStoredTokenSource(provider, store, token.AccountID, opts...), nil
}

// TokenSourceFromStore creates a TokenSource from existing stored credentials.
// If accountID is empty, uses the most recently updated token for the provider.
func TokenSourceFromStore(
	providerName entities.OAuthProvider,
	store *tokenstore.TokenStore,
	accountID string,
	opts ...StoredTokenSourceOption,
) (*StoredTokenSource, error) {
	provider, err := GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	if accountID == "" {
		return ProviderTokenSource(provider, store, opts...)
	}

	return NewStoredTokenSource(provider, store, accountID, opts...), nil
}
