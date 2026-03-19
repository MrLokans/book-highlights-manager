package providers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestProvider creates a DropboxProvider pointing at a test server.
func newTestProvider(t *testing.T, handler http.Handler) (*httptest.Server, *DropboxProvider) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p := NewDropboxProvider("test-app-key")
	p.httpClient = srv.Client()
	// Override URLs to point at test server
	p.tokenURL = srv.URL + "/oauth2/token"
	p.apiURL = srv.URL
	return srv, p
}

// --- Name / Config ---

func TestDropboxProvider_Name(t *testing.T) {
	p := NewDropboxProvider("key")
	assert.Equal(t, entities.OAuthProviderDropbox, p.Name())
}

func TestDropboxProvider_Config(t *testing.T) {
	p := NewDropboxProvider("my-key")
	cfg := p.Config()
	assert.Equal(t, "my-key", cfg.ClientID)
	assert.Equal(t, dropboxAuthURL, cfg.AuthURL)
}

// --- BuildAuthURL ---

func TestBuildAuthURL_ContainsExpectedParams(t *testing.T) {
	p := NewDropboxProvider("my-key")
	authURL, codeVerifier, state, err := p.BuildAuthURL("http://localhost/callback")

	require.NoError(t, err)
	assert.NotEmpty(t, codeVerifier)
	assert.NotEmpty(t, state)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	q := parsed.Query()
	assert.Equal(t, "my-key", q.Get("client_id"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, state, q.Get("state"))
	assert.Equal(t, "offline", q.Get("token_access_type"))
	assert.Equal(t, "http://localhost/callback", q.Get("redirect_uri"))

	// Verify code challenge matches verifier
	hash := sha256.Sum256([]byte(codeVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
	assert.Equal(t, expectedChallenge, q.Get("code_challenge"))
}

func TestBuildAuthURL_NoRedirectURL(t *testing.T) {
	p := NewDropboxProvider("key")
	authURL, _, _, err := p.BuildAuthURL("")

	require.NoError(t, err)
	parsed, _ := url.Parse(authURL)
	assert.Empty(t, parsed.Query().Get("redirect_uri"))
}

// --- ExchangeCode ---

func TestExchangeCode_Success(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/oauth2/token", r.URL.Path)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))
		assert.Equal(t, "authorization_code", params.Get("grant_type"))
		assert.Equal(t, "auth-code", params.Get("code"))
		assert.Equal(t, "test-app-key", params.Get("client_id"))
		assert.Equal(t, "verifier123", params.Get("code_verifier"))
		assert.Equal(t, "http://localhost/cb", params.Get("redirect_uri"))

		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-123",
			"refresh_token": "refresh-456",
			"token_type":    "bearer",
			"expires_in":    14400,
			"scope":         "files.content.read",
			"account_id":    "dbid:AAA",
		})
	}))

	resp, err := p.ExchangeCode(context.Background(), "auth-code", "verifier123", "http://localhost/cb")
	require.NoError(t, err)
	assert.Equal(t, "access-123", resp.AccessToken)
	assert.Equal(t, "refresh-456", resp.RefreshToken)
	assert.Equal(t, "bearer", resp.TokenType)
	assert.Equal(t, 14400, resp.ExpiresIn)
	assert.Equal(t, "dbid:AAA", resp.AccountID)
}

func TestExchangeCode_ErrorResponse(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "code has expired",
		})
	}))

	_, err := p.ExchangeCode(context.Background(), "bad-code", "v", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
	assert.Contains(t, err.Error(), "code has expired")
}

func TestExchangeCode_NonJSONError(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))

	_, err := p.ExchangeCode(context.Background(), "code", "v", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestExchangeCode_ContextCancelled(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		select {} // hang
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.ExchangeCode(ctx, "code", "v", "")
	require.Error(t, err)
}

// --- RefreshToken ---

func TestRefreshToken_Success(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))
		assert.Equal(t, "refresh_token", params.Get("grant_type"))
		assert.Equal(t, "my-refresh", params.Get("refresh_token"))
		assert.Equal(t, "test-app-key", params.Get("client_id"))

		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"token_type":   "bearer",
			"expires_in":   14400,
		})
	}))

	resp, err := p.RefreshToken(context.Background(), "my-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-access", resp.AccessToken)
	assert.Equal(t, "bearer", resp.TokenType)
	assert.Equal(t, 14400, resp.ExpiresIn)
	assert.Empty(t, resp.RefreshToken, "Dropbox doesn't return new refresh token")
}

func TestRefreshToken_Error(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "refresh token is invalid",
		})
	}))

	_, err := p.RefreshToken(context.Background(), "bad-refresh")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_grant")
}

func TestRefreshToken_NonJSONError(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("bad gateway"))
	}))

	_, err := p.RefreshToken(context.Background(), "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

// --- GetAccountInfo ---

func TestGetAccountInfo_Success(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/get_current_account", r.URL.Path)
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		json.NewEncoder(w).Encode(map[string]string{
			"account_id": "dbid:AABBCC",
		})
	}))

	id, err := p.GetAccountInfo(context.Background(), "my-token")
	require.NoError(t, err)
	assert.Equal(t, "dbid:AABBCC", id)
}

func TestGetAccountInfo_Error(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))

	_, err := p.GetAccountInfo(context.Background(), "bad-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestGetAccountInfo_ContextCancelled(t *testing.T) {
	_, p := newTestProvider(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		select {} // hang
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.GetAccountInfo(ctx, "tok")
	require.Error(t, err)
}

// --- PKCE helpers ---

func TestGenerateCodeVerifier(t *testing.T) {
	v1, err := generateCodeVerifier()
	require.NoError(t, err)
	assert.NotEmpty(t, v1)
	assert.GreaterOrEqual(t, len(v1), 43, "verifier should be at least 43 chars for 32 bytes base64url")

	v2, _ := generateCodeVerifier()
	assert.NotEqual(t, v1, v2, "verifiers should be unique")
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-verifier-string"
	challenge := generateCodeChallenge(verifier)

	// Verify it's a valid S256 challenge
	hash := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(hash[:])
	assert.Equal(t, expected, challenge)
}

func TestGenerateState(t *testing.T) {
	s1, err := generateState()
	require.NoError(t, err)
	assert.NotEmpty(t, s1)

	s2, _ := generateState()
	assert.NotEqual(t, s1, s2, "states should be unique")
}

// --- RegisterDropbox ---

func TestRegisterDropbox_EmptyKey(_ *testing.T) {
	// Should not panic or register anything
	RegisterDropbox("")
}
