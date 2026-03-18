package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/database/books"
	"github.com/mrlokans/assistant/internal/database/sources"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBooksTestDB(t *testing.T) (*books.Repository, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := t.TempDir() + "/test_books.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	sourcesRepo := sources.NewRepository(db.DB)
	booksRepo := books.NewRepository(db.DB, sourcesRepo)

	cleanup := func() {
		db.Close()
	}
	return booksRepo, cleanup
}

func TestBooksController_GetAllBooks(t *testing.T) {
	t.Run("returns empty list when no books", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books", controller.GetAllBooks)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(0), response["count"])
		assert.Empty(t, response["books"])
	})

	t.Run("returns books with count", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		// Add some books
		require.NoError(t, repo.SaveBook(&entities.Book{Title: "Book 1", Author: "Author 1"}))
		require.NoError(t, repo.SaveBook(&entities.Book{Title: "Book 2", Author: "Author 2"}))

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books", controller.GetAllBooks)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["count"])
		books := response["books"].([]any)
		assert.Len(t, books, 2)
	})
}

func TestBooksController_GetBookByTitleAndAuthor(t *testing.T) {
	t.Run("returns 400 when title is missing", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books/search", controller.GetBookByTitleAndAuthor)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books/search?author=Author", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "title and author query parameters are required")
	})

	t.Run("returns 400 when author is missing", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books/search", controller.GetBookByTitleAndAuthor)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books/search?title=Title", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("returns 404 when book not found", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books/search", controller.GetBookByTitleAndAuthor)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books/search?title=NonExistent&author=Nobody", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "book not found")
	})

	t.Run("returns book when found", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		require.NoError(t, repo.SaveBook(&entities.Book{Title: "Found Book", Author: "Known Author"}))

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books/search", controller.GetBookByTitleAndAuthor)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books/search?title=Found+Book&author=Known+Author", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var book entities.Book
		err := json.Unmarshal(w.Body.Bytes(), &book)
		require.NoError(t, err)
		assert.Equal(t, "Found Book", book.Title)
		assert.Equal(t, "Known Author", book.Author)
	})
}

func TestBooksController_GetBookStats(t *testing.T) {
	t.Run("returns zero stats when no books", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books/stats", controller.GetBookStats)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(0), response["total_books"])
		assert.Equal(t, float64(0), response["total_highlights"])
	})

	t.Run("returns correct stats", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		require.NoError(t, repo.SaveBook(&entities.Book{
			Title:  "Stats Book 1",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "Highlight 1"},
				{Text: "Highlight 2"},
			},
		}))
		require.NoError(t, repo.SaveBook(&entities.Book{
			Title:  "Stats Book 2",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "Highlight 3"},
			},
		}))

		controller := NewBooksController(repo)

		router := gin.New()
		router.GET("/api/books/stats", controller.GetBookStats)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/books/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(2), response["total_books"])
		assert.Equal(t, float64(3), response["total_highlights"])
	})
}

func TestNewBooksController(t *testing.T) {
	t.Run("creates controller with reader", func(t *testing.T) {
		repo, cleanup := setupBooksTestDB(t)
		defer cleanup()

		controller := NewBooksController(repo)

		assert.NotNil(t, controller)
	})
}
