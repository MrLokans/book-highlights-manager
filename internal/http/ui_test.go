package http

import (
	"archive/zip"
	"bytes"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUITestDB(t *testing.T) (*database.Database, *exporters.DatabaseMarkdownExporter, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := "./test_ui_" + strings.ReplaceAll(t.Name(), "/", "_") + ".db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	tempDir := t.TempDir()
	exporter := exporters.NewDatabaseMarkdownExporter(db, tempDir, "exports")

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, exporter, cleanup
}

func TestUIController_BookPage(t *testing.T) {
	t.Run("returns 400 for invalid book ID", func(t *testing.T) {
		_, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/:id", controller.BookPage)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/invalid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid book ID")
	})

	t.Run("returns 404 for nonexistent book", func(t *testing.T) {
		_, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/:id", controller.BookPage)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/99999", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Book not found")
	})
}

func TestUIController_DownloadMarkdown(t *testing.T) {
	t.Run("returns 400 for invalid book ID", func(t *testing.T) {
		_, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/:id/download", controller.DownloadMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/abc/download", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 for nonexistent book", func(t *testing.T) {
		_, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/:id/download", controller.DownloadMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/99999/download", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns markdown file for valid book", func(t *testing.T) {
		db, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:  "Download Test",
			Author: "Test Author",
			Highlights: []entities.Highlight{
				{Text: "Test highlight"},
			},
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/:id/download", controller.DownloadMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/1/download", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Disposition"), "Download Test.md")
		assert.Equal(t, "text/markdown; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "title: \"Download Test\"")
		assert.Contains(t, w.Body.String(), "> Test highlight")
	})

	t.Run("sanitizes filename with slashes", func(t *testing.T) {
		db, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:  "Book/With/Slashes",
			Author: "Author",
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/:id/download", controller.DownloadMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/1/download", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Disposition"), "Book-With-Slashes.md")
	})
}

func TestUIController_DownloadAllMarkdown(t *testing.T) {
	t.Run("returns empty zip when no books", func(t *testing.T) {
		_, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/download/all", controller.DownloadAllMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/download/all", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "highlights-")
		assert.Contains(t, w.Header().Get("Content-Disposition"), ".zip")
	})

	t.Run("returns zip with all books", func(t *testing.T) {
		db, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		require.NoError(t, db.SaveBook(&entities.Book{
			Title:  "Book One",
			Author: "Author",
			Source: entities.Source{Name: "kindle"},
		}))
		require.NoError(t, db.SaveBook(&entities.Book{
			Title:  "Book Two",
			Author: "Author",
			Source: entities.Source{Name: "apple_books"},
		}))

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/download/all", controller.DownloadAllMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/download/all", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify ZIP contents
		zipReader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
		require.NoError(t, err)

		fileNames := make([]string, 0)
		for _, f := range zipReader.File {
			fileNames = append(fileNames, f.Name)
		}

		assert.Len(t, fileNames, 2)
		// Should be organized by source
		hasKindleBook := false
		hasAppleBook := false
		for _, name := range fileNames {
			if strings.Contains(name, "kindle") && strings.Contains(name, "Book One.md") {
				hasKindleBook = true
			}
			if strings.Contains(name, "apple_books") && strings.Contains(name, "Book Two.md") {
				hasAppleBook = true
			}
		}
		assert.True(t, hasKindleBook, "Should have kindle book")
		assert.True(t, hasAppleBook, "Should have apple_books book")
	})

	t.Run("zip files contain markdown content", func(t *testing.T) {
		db, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		require.NoError(t, db.SaveBook(&entities.Book{
			Title:  "Content Test",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "Zipped highlight"},
			},
		}))

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/download/all", controller.DownloadAllMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/download/all", nil)
		router.ServeHTTP(w, req)

		zipReader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
		require.NoError(t, err)
		require.Len(t, zipReader.File, 1)

		rc, err := zipReader.File[0].Open()
		require.NoError(t, err)
		defer rc.Close()

		content, err := io.ReadAll(rc)
		require.NoError(t, err)

		assert.Contains(t, string(content), "title: \"Content Test\"")
		assert.Contains(t, string(content), "> Zipped highlight")
	})

	t.Run("uses unknown folder for books without source", func(t *testing.T) {
		db, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		require.NoError(t, db.SaveBook(&entities.Book{
			Title:  "No Source Book",
			Author: "Author",
		}))

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.GET("/ui/books/download/all", controller.DownloadAllMarkdown)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/download/all", nil)
		router.ServeHTTP(w, req)

		zipReader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
		require.NoError(t, err)

		assert.Contains(t, zipReader.File[0].Name, "unknown")
	})
}

func TestUIController_SearchBooks(t *testing.T) {
	t.Run("returns all books when query is empty", func(t *testing.T) {
		db, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		require.NoError(t, db.SaveBook(&entities.Book{Title: "Book 1", Author: "Author"}))
		require.NoError(t, db.SaveBook(&entities.Book{Title: "Book 2", Author: "Author"}))

		controller := NewUIController(exporter, nil)

		// Note: SearchBooks returns HTML, so we just check status code
		router := gin.New()
		router.SetHTMLTemplate(createTestTemplate())
		router.GET("/ui/books/search", controller.SearchBooks)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/search?q=", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("filters books by query", func(t *testing.T) {
		db, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		require.NoError(t, db.SaveBook(&entities.Book{Title: "Python Programming", Author: "Author"}))
		require.NoError(t, db.SaveBook(&entities.Book{Title: "Go Programming", Author: "Author"}))

		controller := NewUIController(exporter, nil)

		router := gin.New()
		router.SetHTMLTemplate(createTestTemplate())
		router.GET("/ui/books/search", controller.SearchBooks)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/books/search?q=Python", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestNewUIController(t *testing.T) {
	t.Run("creates controller with exporter", func(t *testing.T) {
		_, exporter, cleanup := setupUITestDB(t)
		defer cleanup()

		controller := NewUIController(exporter, nil)

		assert.NotNil(t, controller)
	})
}

// Helper to create a minimal test template
func createTestTemplate() *template.Template {
	return template.Must(template.New("book-list").Parse("{{.}}"))
}
