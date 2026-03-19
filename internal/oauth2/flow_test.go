package oauth2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlowHandler(t *testing.T) {
	p := &mockProvider{name: "test"}
	fh := NewFlowHandler(p, nil)
	assert.NotNil(t, fh)
}

func TestDefaultCLIFlowConfig(t *testing.T) {
	cfg := DefaultCLIFlowConfig()
	assert.Equal(t, 8089, cfg.Port)
	assert.NotNil(t, cfg.OnAuthURL)
	assert.NotNil(t, cfg.OnCodeReceived)
	assert.NotNil(t, cfg.OnTokenReceived)
	assert.NotNil(t, cfg.OnError)
}

func TestGetManualAuthURL(t *testing.T) {
	p := &mockProvider{name: "test"}
	fh := NewFlowHandler(p, nil)

	authURL, codeVerifier, err := fh.GetManualAuthURL()
	require.NoError(t, err)
	assert.Equal(t, "https://auth", authURL)
	assert.Equal(t, "verifier", codeVerifier)
}

func TestStartWebFlow(t *testing.T) {
	p := &mockProvider{name: "test"}
	fh := NewFlowHandler(p, nil)

	authURL, state, codeVerifier, err := fh.StartWebFlow(WebFlowConfig{
		RedirectURL: "http://localhost/cb",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://auth", authURL)
	assert.Equal(t, "state", state)
	assert.Equal(t, "verifier", codeVerifier)
}

func TestStartWebFlow_CustomState(t *testing.T) {
	p := &mockProvider{name: "test"}
	fh := NewFlowHandler(p, nil)

	_, state, _, err := fh.StartWebFlow(WebFlowConfig{
		RedirectURL: "http://localhost/cb",
		State:       "custom-state",
	})
	require.NoError(t, err)
	assert.Equal(t, "custom-state", state)
}

func TestCompleteWebFlow_StateMismatch(t *testing.T) {
	p := &mockProvider{name: "test"}
	fh := NewFlowHandler(p, nil)

	_, err := fh.CompleteWebFlow(context.Background(), "code", "verifier", "http://localhost/cb", "expected", "wrong")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state mismatch")
}

func TestCompleteWebFlow_Success_NoStore(t *testing.T) {
	p := &mockProvider{name: "test"}
	fh := NewFlowHandler(p, nil)

	result, err := fh.CompleteWebFlow(context.Background(), "code", "verifier", "http://localhost/cb", "state", "state")
	require.NoError(t, err)
	assert.Equal(t, "tok", result.AccessToken)
	assert.Equal(t, "account-123", result.AccountID)
}

func TestCompleteWebFlow_EmptyExpectedState(t *testing.T) {
	p := &mockProvider{name: "test"}
	fh := NewFlowHandler(p, nil)

	// Empty expected state skips verification
	result, err := fh.CompleteWebFlow(context.Background(), "code", "verifier", "", "", "anything")
	require.NoError(t, err)
	assert.Equal(t, "tok", result.AccessToken)
}

func TestShutdownCtx(t *testing.T) {
	ctx := shutdownCtx(context.Background())
	assert.NotNil(t, ctx)
	// Should have a deadline
	_, ok := ctx.Deadline()
	assert.True(t, ok)
}

// --- Global registry helpers ---

func TestGlobalRegisterAndGetProvider(t *testing.T) {
	// Save and restore the default registry
	orig := DefaultRegistry
	defer func() { DefaultRegistry = orig }()
	DefaultRegistry = NewRegistry()

	p := &mockProvider{name: "global-test"}
	Register(p)

	got, err := GetProvider("global-test")
	require.NoError(t, err)
	assert.Equal(t, p.Name(), got.Name())
}

func TestGetProvider_NotFound(t *testing.T) {
	orig := DefaultRegistry
	defer func() { DefaultRegistry = orig }()
	DefaultRegistry = NewRegistry()

	_, err := GetProvider("missing")
	require.Error(t, err)
}
