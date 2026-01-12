package covers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Cache handles local caching of book cover images.
type Cache struct {
	cacheDir   string
	httpClient *http.Client
}

// NewCache creates a new cover cache at the specified directory.
func NewCache(cacheDir string) (*Cache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	return &Cache{
		cacheDir: cacheDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// GetCover returns the cached cover for a book, or fetches and caches it if not present.
// Returns the file path to the cached cover, or empty string if unavailable.
func (c *Cache) GetCover(bookID uint, coverURL string) (string, error) {
	if coverURL == "" {
		return "", nil
	}

	filename := c.coverFilename(bookID, coverURL)
	cachePath := filepath.Join(c.cacheDir, filename)

	// Check if cached file exists
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// Fetch and cache the cover
	if err := c.fetchAndCache(coverURL, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

// InvalidateCover removes the cached cover for a book.
func (c *Cache) InvalidateCover(bookID uint) error {
	pattern := filepath.Join(c.cacheDir, fmt.Sprintf("cover_%d_*", bookID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

// coverFilename generates a unique filename based on book ID and URL hash.
func (c *Cache) coverFilename(bookID uint, coverURL string) string {
	hash := sha256.Sum256([]byte(coverURL))
	return fmt.Sprintf("cover_%d_%x.jpg", bookID, hash[:8])
}

// fetchAndCache downloads a cover image and saves it to the cache.
func (c *Cache) fetchAndCache(url, cachePath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "HighlightsManager/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch cover: status %d", resp.StatusCode)
	}

	// Create temp file in same directory for atomic write
	tmpFile, err := os.CreateTemp(c.cacheDir, "cover_tmp_")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath) // Clean up if we didn't rename
	}()

	// Copy response body to temp file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return err
	}

	tmpFile.Close()

	// Atomic rename
	return os.Rename(tmpPath, cachePath)
}

// CacheDir returns the cache directory path.
func (c *Cache) CacheDir() string {
	return c.cacheDir
}
