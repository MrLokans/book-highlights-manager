package dropbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staticTokenSource implements oauth2.TokenSource for tests.
type staticTokenSource struct {
	token string
	err   error
}

func (s *staticTokenSource) Token(_ context.Context) (string, error) { return s.token, s.err }
func (s *staticTokenSource) ForceRefresh(_ context.Context) error    { return nil }
func (s *staticTokenSource) IsValid() bool                           { return s.err == nil }
func (s *staticTokenSource) ExpiresAt() *time.Time                   { return nil }
func (s *staticTokenSource) AccountID() string                       { return "test-account" }

// newTestClient creates an httptest server and a Client pointed at it.
func newTestClient(t *testing.T, handler http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClientWithOptions(
		&staticTokenSource{token: "test-token"},
		WithURLs(srv.URL, srv.URL),
		WithHTTPClient(srv.Client()),
	)
	return srv, client
}

// --- List ---

func TestList_Success(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "/files/list_folder", r.URL.Path)

		json.NewEncoder(w).Encode(map[string]any{
			"entries": []map[string]any{
				{".tag": "file", "name": "doc.txt", "path_display": "/doc.txt", "size": 100},
				{".tag": "folder", "name": "subdir", "path_display": "/subdir"},
			},
			"has_more": false,
		})
	}))

	files, err := client.List(context.Background(), "/")
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "doc.txt", files[0].Name)
	assert.False(t, files[0].IsDir)
	assert.Equal(t, int64(100), files[0].Size)
	assert.Equal(t, "subdir", files[1].Name)
	assert.True(t, files[1].IsDir)
}

func TestList_Pagination(t *testing.T) {
	callCount := 0
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/files/list_folder":
			json.NewEncoder(w).Encode(map[string]any{
				"entries":  []map[string]any{{".tag": "file", "name": "a.txt", "path_display": "/a.txt"}},
				"cursor":   "cursor123",
				"has_more": true,
			})
		case "/files/list_folder/continue":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			assert.Equal(t, "cursor123", body["cursor"])
			json.NewEncoder(w).Encode(map[string]any{
				"entries":  []map[string]any{{".tag": "file", "name": "b.txt", "path_display": "/b.txt"}},
				"has_more": false,
			})
		}
	}))

	files, err := client.List(context.Background(), "/")
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Equal(t, 2, callCount)
}

func TestList_APIError(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))

	_, err := client.List(context.Background(), "/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestList_TokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	client := NewClientWithOptions(
		&staticTokenSource{err: assert.AnError},
		WithURLs(srv.URL, srv.URL),
	)

	_, err := client.List(context.Background(), "/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get token")
}

// --- Download ---

func TestDownload_Success(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/files/download", r.URL.Path)
		assert.Contains(t, r.Header.Get("Dropbox-API-Arg"), "test/file.txt")
		w.Write([]byte("file contents"))
	}))

	reader, err := client.Download(context.Background(), "test/file.txt")
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "file contents", string(data))
}

func TestDownload_APIError(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))

	_, err := client.Download(context.Background(), "/missing.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestDownload_TokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	client := NewClientWithOptions(
		&staticTokenSource{err: assert.AnError},
		WithURLs(srv.URL, srv.URL),
	)

	_, err := client.Download(context.Background(), "/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get token")
}

// --- Upload ---

// --- Delete ---

// --- Exists ---

// --- GetMetadata ---

// --- APIError / IsNotFound ---

func TestAPIError_Error(t *testing.T) {
	err := &APIError{StatusCode: 409, Body: "conflict"}
	assert.Contains(t, err.Error(), "409")
	assert.Contains(t, err.Error(), "conflict")
}

// --- Options ---

func TestNewClient_Defaults(t *testing.T) {
	ts := &staticTokenSource{token: "tok"}
	c := NewClient(ts)
	assert.Equal(t, defaultAPIURL, c.apiURL)
	assert.Equal(t, defaultContentURL, c.contentURL)
	assert.NotNil(t, c.httpClient)
}

func TestNewClientWithOptions_URLs(t *testing.T) {
	ts := &staticTokenSource{token: "tok"}
	c := NewClientWithOptions(ts, WithURLs("http://api", "http://content"))
	assert.Equal(t, "http://api", c.apiURL)
	assert.Equal(t, "http://content", c.contentURL)
}

func TestNewClientWithOptions_HTTPClient(t *testing.T) {
	ts := &staticTokenSource{token: "tok"}
	custom := &http.Client{}
	c := NewClientWithOptions(ts, WithHTTPClient(custom))
	assert.Same(t, custom, c.httpClient)
}

// --- Context cancellation ---

func TestList_ContextCancelled(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Hang forever — context should cancel
		select {}
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.List(ctx, "/")
	require.Error(t, err)
}
