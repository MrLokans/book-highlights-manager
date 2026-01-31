package oauth2

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/tokenstore"
)

// RefreshConfig contains configuration for the token refresh scheduler
type RefreshConfig struct {
	Enabled       bool          // Enable background refresh
	CheckInterval time.Duration // How often to check for expiring tokens (default: 30m)
	RefreshMargin time.Duration // Refresh tokens expiring within this duration (default: 15m)
}

// DefaultRefreshConfig returns sensible defaults for token refresh
func DefaultRefreshConfig() RefreshConfig {
	return RefreshConfig{
		Enabled:       true,
		CheckInterval: 30 * time.Minute,
		RefreshMargin: 15 * time.Minute,
	}
}

// RefreshScheduler handles background token refresh for all registered providers
type RefreshScheduler struct {
	mu sync.Mutex

	tokenStore   *tokenstore.TokenStore
	registry     *Registry
	config       RefreshConfig
	auditService *audit.Service

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewRefreshScheduler creates a new token refresh scheduler
func NewRefreshScheduler(
	store *tokenstore.TokenStore,
	registry *Registry,
	config RefreshConfig,
	auditService *audit.Service,
) *RefreshScheduler {
	if registry == nil {
		registry = DefaultRegistry
	}

	return &RefreshScheduler{
		tokenStore:   store,
		registry:     registry,
		config:       config,
		auditService: auditService,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// Start begins the background refresh scheduler
func (s *RefreshScheduler) Start(ctx context.Context) {
	if !s.config.Enabled {
		log.Println("OAuth2 token refresh scheduler disabled")
		close(s.doneCh)
		return
	}

	log.Printf("OAuth2 token refresh scheduler started (interval: %v, margin: %v)",
		s.config.CheckInterval, s.config.RefreshMargin)

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	// Run an initial check
	s.refreshExpiringTokens(ctx)

	for {
		select {
		case <-ticker.C:
			s.refreshExpiringTokens(ctx)
		case <-s.stopCh:
			log.Println("OAuth2 token refresh scheduler stopping")
			close(s.doneCh)
			return
		case <-ctx.Done():
			log.Println("OAuth2 token refresh scheduler context cancelled")
			close(s.doneCh)
			return
		}
	}
}

// Stop gracefully stops the scheduler
func (s *RefreshScheduler) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *RefreshScheduler) refreshExpiringTokens(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	providers := s.registry.All()
	if len(providers) == 0 {
		return
	}

	for _, provider := range providers {
		if err := s.refreshProviderTokens(ctx, provider); err != nil {
			log.Printf("Error refreshing %s tokens: %v", provider.Name(), err)
		}
	}
}

func (s *RefreshScheduler) refreshProviderTokens(ctx context.Context, provider Provider) error {
	tokens, err := s.tokenStore.ListTokens(provider.Name())
	if err != nil {
		return err
	}

	for _, token := range tokens {
		if !token.IsExpiringSoon(s.config.RefreshMargin) {
			continue
		}

		// Need to get decrypted token for refresh
		decrypted, err := s.tokenStore.GetToken(provider.Name(), token.AccountID)
		if err != nil {
			log.Printf("Failed to get token for %s/%s: %v", provider.Name(), token.AccountID, err)
			s.logAudit("oauth_token_refresh",
				fmt.Sprintf("Failed to get %s token for %s", provider.Name(), token.AccountID), err)
			continue
		}

		if decrypted.RefreshToken == "" {
			log.Printf("No refresh token available for %s/%s", provider.Name(), token.AccountID)
			s.logAudit("oauth_token_refresh",
				fmt.Sprintf("No refresh token for %s/%s", provider.Name(), token.AccountID),
				fmt.Errorf("no refresh token available"))
			continue
		}

		log.Printf("Refreshing expiring token for %s/%s", provider.Name(), token.AccountID)

		resp, err := provider.RefreshToken(ctx, decrypted.RefreshToken)
		if err != nil {
			log.Printf("Failed to refresh token for %s/%s: %v", provider.Name(), token.AccountID, err)
			s.logAudit("oauth_token_refresh",
				fmt.Sprintf("Failed to refresh %s token for %s", provider.Name(), token.AccountID), err)
			continue
		}

		// Update the stored token
		newRefreshToken := resp.RefreshToken
		if newRefreshToken == "" {
			newRefreshToken = decrypted.RefreshToken
		}

		if err := s.tokenStore.UpdateTokenAfterRefresh(
			provider.Name(),
			token.AccountID,
			resp.AccessToken,
			newRefreshToken,
			resp.ExpiresAt(),
		); err != nil {
			log.Printf("Failed to save refreshed token for %s/%s: %v", provider.Name(), token.AccountID, err)
			s.logAudit("oauth_token_refresh",
				fmt.Sprintf("Failed to save refreshed %s token for %s", provider.Name(), token.AccountID), err)
			continue
		}

		log.Printf("Successfully refreshed token for %s/%s", provider.Name(), token.AccountID)
		s.logAudit("oauth_token_refresh",
			fmt.Sprintf("Refreshed %s token for %s", provider.Name(), token.AccountID), nil)
	}

	return nil
}

func (s *RefreshScheduler) logAudit(action, description string, err error) {
	if s.auditService == nil {
		return
	}
	s.auditService.LogSync(0, action, description, err)
}

// RefreshToken manually triggers a refresh for a specific token
func (s *RefreshScheduler) RefreshToken(ctx context.Context, providerName entities.OAuthProvider, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, err := s.registry.Get(providerName)
	if err != nil {
		return err
	}

	token, err := s.tokenStore.GetToken(providerName, accountID)
	if err != nil {
		return err
	}
	if token == nil {
		return ErrTokenNotFound
	}
	if token.RefreshToken == "" {
		return ErrNoRefreshToken
	}

	resp, err := provider.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		return err
	}

	newRefreshToken := resp.RefreshToken
	if newRefreshToken == "" {
		newRefreshToken = token.RefreshToken
	}

	return s.tokenStore.UpdateTokenAfterRefresh(
		providerName,
		accountID,
		resp.AccessToken,
		newRefreshToken,
		resp.ExpiresAt(),
	)
}
