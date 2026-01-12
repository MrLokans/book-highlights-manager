package covers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCache(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "covers")

	cache, err := NewCache(cacheDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	if cache.CacheDir() != cacheDir {
		t.Errorf("expected cache dir %s, got %s", cacheDir, cache.CacheDir())
	}

	// Verify directory was created
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("cache directory was not created")
	}
}

func TestGetCover_EmptyURL(t *testing.T) {
	cache, _ := NewCache(t.TempDir())

	path, err := cache.GetCover(1, "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for empty URL, got %s", path)
	}
}

func TestGetCover_FetchAndCache(t *testing.T) {
	// Create a test server that serves an image
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	cache, _ := NewCache(t.TempDir())

	// First request should fetch
	path1, err := cache.GetCover(1, server.URL+"/cover.jpg")
	if err != nil {
		t.Fatalf("GetCover failed: %v", err)
	}
	if path1 == "" {
		t.Fatal("expected non-empty path")
	}

	// Verify file exists
	if _, err := os.Stat(path1); os.IsNotExist(err) {
		t.Error("cached file does not exist")
	}

	// Second request should use cache
	path2, err := cache.GetCover(1, server.URL+"/cover.jpg")
	if err != nil {
		t.Fatalf("GetCover (cached) failed: %v", err)
	}
	if path1 != path2 {
		t.Error("expected same path for cached request")
	}
}

func TestGetCover_FetchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cache, _ := NewCache(t.TempDir())

	_, err := cache.GetCover(1, server.URL+"/notfound.jpg")
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestInvalidateCover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	cache, _ := NewCache(t.TempDir())

	// Fetch and cache a cover
	path, err := cache.GetCover(1, server.URL+"/cover.jpg")
	if err != nil {
		t.Fatalf("GetCover failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("cached file does not exist")
	}

	// Invalidate
	err = cache.InvalidateCover(1)
	if err != nil {
		t.Fatalf("InvalidateCover failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cached file should be deleted after invalidation")
	}
}

func TestCoverFilename(t *testing.T) {
	cache, _ := NewCache(t.TempDir())

	// Same URL should give same filename
	name1 := cache.coverFilename(1, "https://example.com/cover.jpg")
	name2 := cache.coverFilename(1, "https://example.com/cover.jpg")
	if name1 != name2 {
		t.Error("same inputs should produce same filename")
	}

	// Different URL should give different filename
	name3 := cache.coverFilename(1, "https://example.com/other.jpg")
	if name1 == name3 {
		t.Error("different URLs should produce different filenames")
	}

	// Different book ID should give different filename
	name4 := cache.coverFilename(2, "https://example.com/cover.jpg")
	if name1 == name4 {
		t.Error("different book IDs should produce different filenames")
	}
}
