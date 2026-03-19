package dropbox

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestUpload_Success(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/files/upload", r.URL.Path)
		assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Dropbox-API-Arg"), "/upload/test.txt")

		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "upload data", string(body))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"name": "test.txt"})
	}))

	err := client.Upload(context.Background(), "/upload/test.txt", strings.NewReader("upload data"))
	assert.NoError(t, err)
}

func TestUpload_APIError(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInsufficientStorage)
		w.Write([]byte("quota exceeded"))
	}))

	err := client.Upload(context.Background(), "/file.txt", strings.NewReader("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "507")
}

func TestUpload_TokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	client := NewClientWithOptions(
		&staticTokenSource{err: assert.AnError},
		WithURLs(srv.URL, srv.URL),
	)

	err := client.Upload(context.Background(), "/file.txt", strings.NewReader("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get token")
}

// --- Delete ---

func TestDelete_Success(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/files/delete_v2", r.URL.Path)

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "/delete-me.txt", body["path"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))

	err := client.Delete(context.Background(), "/delete-me.txt")
	assert.NoError(t, err)
}

func TestDelete_APIError(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte("conflict"))
	}))

	err := client.Delete(context.Background(), "/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "409")
}

func TestDelete_TokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	client := NewClientWithOptions(
		&staticTokenSource{err: assert.AnError},
		WithURLs(srv.URL, srv.URL),
	)

	err := client.Delete(context.Background(), "/file.txt")
	require.Error(t, err)
}

// --- Exists ---

func TestExists_True(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			".tag":         "file",
			"name":         "exists.txt",
			"path_display": "/exists.txt",
		})
	}))

	exists, err := client.Exists(context.Background(), "/exists.txt")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestExists_False_NotFound(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": {"path": {".tag": "not_found"}}}`))
	}))

	exists, err := client.Exists(context.Background(), "/missing.txt")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestExists_Error(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))

	_, err := client.Exists(context.Background(), "/file.txt")
	require.Error(t, err)
}

// --- GetMetadata ---

func TestGetMetadata_File(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/files/get_metadata", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{
			".tag":         "file",
			"name":         "readme.md",
			"path_display": "/readme.md",
			"id":           "id:abc123",
			"size":         42,
			"content_hash": "hash123",
		})
	}))

	info, err := client.GetMetadata(context.Background(), "/readme.md")
	require.NoError(t, err)
	assert.Equal(t, "readme.md", info.Name)
	assert.Equal(t, "/readme.md", info.Path)
	assert.False(t, info.IsDir)
	assert.Equal(t, int64(42), info.Size)
	assert.Equal(t, "id:abc123", info.ID)
	assert.Equal(t, "hash123", info.ContentHash)
}

func TestGetMetadata_Folder(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			".tag":         "folder",
			"name":         "docs",
			"path_display": "/docs",
		})
	}))

	info, err := client.GetMetadata(context.Background(), "/docs")
	require.NoError(t, err)
	assert.True(t, info.IsDir)
	assert.Equal(t, "docs", info.Name)
}

func TestGetMetadata_NotFound(t *testing.T) {
	_, client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error": {"path": {".tag": "not_found"}}}`))
	}))

	_, err := client.GetMetadata(context.Background(), "/missing.txt")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
}

func TestGetMetadata_TokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	client := NewClientWithOptions(
		&staticTokenSource{err: assert.AnError},
		WithURLs(srv.URL, srv.URL),
	)

	_, err := client.GetMetadata(context.Background(), "/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get token")
}

// --- APIError / IsNotFound ---

func TestAPIError_Error(t *testing.T) {
	err := &APIError{StatusCode: 409, Body: "conflict"}
	assert.Contains(t, err.Error(), "409")
	assert.Contains(t, err.Error(), "conflict")
}

func TestIsNotFound_True(t *testing.T) {
	err := &APIError{StatusCode: 409, Body: `{"error": {"path": {".tag": "not_found"}}}`}
	assert.True(t, IsNotFound(err))
}

func TestIsNotFound_False_DifferentStatus(t *testing.T) {
	err := &APIError{StatusCode: 500, Body: "server error"}
	assert.False(t, IsNotFound(err))
}

func TestIsNotFound_False_DifferentBody(t *testing.T) {
	err := &APIError{StatusCode: 409, Body: `{"error": {"path": {".tag": "malformed_path"}}}`}
	assert.False(t, IsNotFound(err))
}

func TestIsNotFound_NonAPIError(t *testing.T) {
	assert.False(t, IsNotFound(assert.AnError))
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
