package oauth2

import (
	"context"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider implements Provider for testing.
type mockProvider struct {
	name entities.OAuthProvider
}

func (m *mockProvider) Name() entities.OAuthProvider { return m.name }
func (m *mockProvider) Config() ProviderConfig       { return ProviderConfig{ClientID: "test"} }
func (m *mockProvider) BuildAuthURL(_ string) (string, string, string, error) {
	return "https://auth", "verifier", "state", nil
}
func (m *mockProvider) ExchangeCode(_ context.Context, _, _, _ string) (*TokenResponse, error) {
	return &TokenResponse{AccessToken: "tok"}, nil
}
func (m *mockProvider) RefreshToken(_ context.Context, _ string) (*TokenResponse, error) {
	return &TokenResponse{AccessToken: "new-tok"}, nil
}
func (m *mockProvider) GetAccountInfo(_ context.Context, _ string) (string, error) {
	return "account-123", nil
}

// --- TokenResponse ---

func TestTokenResponse_ExpiresAt_WithExpiry(t *testing.T) {
	resp := &TokenResponse{ExpiresIn: 3600}
	exp := resp.ExpiresAt()
	require.NotNil(t, exp)
	// Should be roughly 1 hour from now
	assert.WithinDuration(t, time.Now().Add(time.Hour), *exp, 5*time.Second)
}

func TestTokenResponse_ExpiresAt_Zero(t *testing.T) {
	resp := &TokenResponse{ExpiresIn: 0}
	assert.Nil(t, resp.ExpiresAt())
}

func TestTokenResponse_ExpiresAt_Negative(t *testing.T) {
	resp := &TokenResponse{ExpiresIn: -1}
	assert.Nil(t, resp.ExpiresAt())
}

// --- Registry ---

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	provider := &mockProvider{name: "test-provider"}

	reg.Register(provider)

	got, err := reg.Get("test-provider")
	require.NoError(t, err)
	assert.Equal(t, entities.OAuthProvider("test-provider"), got.Name())
}

func TestRegistry_Get_NotFound(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Get("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "provider-a"})
	reg.Register(&mockProvider{name: "provider-b"})

	names := reg.List()
	assert.Len(t, names, 2)
	assert.Contains(t, names, entities.OAuthProvider("provider-a"))
	assert.Contains(t, names, entities.OAuthProvider("provider-b"))
}

func TestRegistry_List_Empty(t *testing.T) {
	reg := NewRegistry()
	assert.Empty(t, reg.List())
}

func TestRegistry_All(t *testing.T) {
	reg := NewRegistry()
	p1 := &mockProvider{name: "a"}
	p2 := &mockProvider{name: "b"}
	reg.Register(p1)
	reg.Register(p2)

	all := reg.All()
	assert.Len(t, all, 2)
}

func TestRegistry_All_Empty(t *testing.T) {
	reg := NewRegistry()
	assert.Empty(t, reg.All())
}

func TestRegistry_OverwriteProvider(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "dup"})
	reg.Register(&mockProvider{name: "dup"})

	assert.Len(t, reg.List(), 1)
}
