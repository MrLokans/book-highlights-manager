package oauth2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- StaticTokenSource ---

func TestStaticTokenSource_Token(t *testing.T) {
	ts := NewStaticTokenSource("my-access-token", "acct-1")
	tok, err := ts.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "my-access-token", tok)
}

func TestStaticTokenSource_AccountID(t *testing.T) {
	ts := NewStaticTokenSource("tok", "acct-42")
	assert.Equal(t, "acct-42", ts.AccountID())
}

func TestStaticTokenSource_IsValid_True(t *testing.T) {
	ts := NewStaticTokenSource("tok", "acct")
	assert.True(t, ts.IsValid())
}

func TestStaticTokenSource_IsValid_Empty(t *testing.T) {
	ts := NewStaticTokenSource("", "acct")
	assert.False(t, ts.IsValid())
}

func TestStaticTokenSource_ExpiresAt_Nil(t *testing.T) {
	ts := NewStaticTokenSource("tok", "acct")
	assert.Nil(t, ts.ExpiresAt())
}

func TestStaticTokenSource_ForceRefresh_Error(t *testing.T) {
	ts := NewStaticTokenSource("tok", "acct")
	err := ts.ForceRefresh(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support refresh")
}

// --- StoredTokenSource (unit tests without real store) ---

func TestNewStoredTokenSource_Defaults(t *testing.T) {
	provider := &mockProvider{name: "test"}
	ts := NewStoredTokenSource(provider, nil, "acct-1")
	assert.Equal(t, "acct-1", ts.AccountID())
	assert.Equal(t, 5*time.Minute, ts.refreshMargin)
}

func TestNewStoredTokenSource_WithRefreshMargin(t *testing.T) {
	provider := &mockProvider{name: "test"}
	ts := NewStoredTokenSource(provider, nil, "acct-1", WithRefreshMargin(10*time.Minute))
	assert.Equal(t, 10*time.Minute, ts.refreshMargin)
}

func TestStoredTokenSource_IsValid_NoToken(t *testing.T) {
	provider := &mockProvider{name: "test"}
	ts := NewStoredTokenSource(provider, nil, "acct-1")
	assert.False(t, ts.IsValid())
}

func TestStoredTokenSource_ExpiresAt_Nil(t *testing.T) {
	provider := &mockProvider{name: "test"}
	ts := NewStoredTokenSource(provider, nil, "acct-1")
	assert.Nil(t, ts.ExpiresAt())
}

func TestStoredTokenSource_IsExpiringSoon(t *testing.T) {
	provider := &mockProvider{name: "test"}
	ts := NewStoredTokenSource(provider, nil, "acct-1", WithRefreshMargin(5*time.Minute))

	// No expiry = not expiring
	ts.accessToken = "tok"
	ts.expiresAt = nil
	assert.True(t, ts.IsValid())

	// Expiry far in the future
	future := time.Now().Add(1 * time.Hour)
	ts.expiresAt = &future
	assert.True(t, ts.IsValid())

	// Expiry within margin
	soon := time.Now().Add(2 * time.Minute)
	ts.expiresAt = &soon
	assert.False(t, ts.IsValid())

	// Already expired
	past := time.Now().Add(-1 * time.Hour)
	ts.expiresAt = &past
	assert.False(t, ts.IsValid())
}

// --- DefaultRefreshConfig ---

func TestDefaultRefreshConfig(t *testing.T) {
	cfg := DefaultRefreshConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30*time.Minute, cfg.CheckInterval)
	assert.Equal(t, 15*time.Minute, cfg.RefreshMargin)
}
