package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTagsTestDB(t *testing.T) (*database.Database, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := "./test_tags_" + strings.ReplaceAll(t.Name(), "/", "_") + ".db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, cleanup
}

func TestTagsController_GetAllTags(t *testing.T) {
	t.Run("returns empty list when no tags exist", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		controller := NewTagsController(db)
		router := gin.New()
		router.GET("/api/tags", controller.GetAllTags)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tags", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "[]", strings.TrimSpace(w.Body.String()))
	})

	t.Run("returns existing tags", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		_, err := db.CreateTag("fiction", 0)
		require.NoError(t, err)
		_, err = db.CreateTag("science", 0)
		require.NoError(t, err)

		controller := NewTagsController(db)
		router := gin.New()
		router.GET("/api/tags", controller.GetAllTags)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tags", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var tags []entities.Tag
		err = json.Unmarshal(w.Body.Bytes(), &tags)
		require.NoError(t, err)
		assert.Len(t, tags, 2)
	})
}

func TestTagsController_CreateTag(t *testing.T) {
	t.Run("creates a new tag", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		controller := NewTagsController(db)
		router := gin.New()
		router.POST("/api/tags", controller.CreateTag)

		body := bytes.NewBufferString(`{"name": "philosophy"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tags", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var tag entities.Tag
		err := json.Unmarshal(w.Body.Bytes(), &tag)
		require.NoError(t, err)
		assert.Equal(t, "philosophy", tag.Name)
		assert.Greater(t, tag.ID, uint(0))
	})

	t.Run("returns existing tag if name exists", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		existingTag, err := db.CreateTag("existing", 0)
		require.NoError(t, err)

		controller := NewTagsController(db)
		router := gin.New()
		router.POST("/api/tags", controller.CreateTag)

		body := bytes.NewBufferString(`{"name": "existing"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tags", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var tag entities.Tag
		err = json.Unmarshal(w.Body.Bytes(), &tag)
		require.NoError(t, err)
		assert.Equal(t, existingTag.ID, tag.ID)
	})

	t.Run("returns error for missing name", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		controller := NewTagsController(db)
		router := gin.New()
		router.POST("/api/tags", controller.CreateTag)

		body := bytes.NewBufferString(`{}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tags", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTagsController_DeleteTag(t *testing.T) {
	t.Run("deletes existing tag", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		tag, err := db.CreateTag("to-delete", 0)
		require.NoError(t, err)

		controller := NewTagsController(db)
		router := gin.New()
		router.DELETE("/api/tags/:id", controller.DeleteTag)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/tags/"+string(rune(tag.ID+'0')), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify tag was deleted
		tags, _ := db.GetTagsForUser(0)
		assert.Empty(t, tags)
	})

	t.Run("returns error for invalid tag ID", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		controller := NewTagsController(db)
		router := gin.New()
		router.DELETE("/api/tags/:id", controller.DeleteTag)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/tags/invalid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTagsController_AddTagToBook(t *testing.T) {
	t.Run("adds tag to book by tag name via JSON", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		book := &entities.Book{Title: "Test Book", Author: "Author"}
		require.NoError(t, db.SaveBook(book))

		controller := NewTagsController(db)
		router := gin.New()
		router.POST("/api/books/:id/tags", controller.AddTagToBook)

		body := bytes.NewBufferString(`{"tag_name": "fiction"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/books/1/tags", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify tag was added
		updatedBook, err := db.GetBookByID(1)
		require.NoError(t, err)
		assert.Len(t, updatedBook.Tags, 1)
		assert.Equal(t, "fiction", updatedBook.Tags[0].Name)
	})

	t.Run("adds tag to book by tag name via form data (HTMX)", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		book := &entities.Book{Title: "Test Book", Author: "Author"}
		require.NoError(t, db.SaveBook(book))

		controller := NewTagsController(db)
		router := gin.New()
		router.POST("/api/books/:id/tags", controller.AddTagToBook)

		body := bytes.NewBufferString("tag_name=nonfiction")
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/books/1/tags", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify tag was added
		updatedBook, err := db.GetBookByID(1)
		require.NoError(t, err)
		assert.Len(t, updatedBook.Tags, 1)
		assert.Equal(t, "nonfiction", updatedBook.Tags[0].Name)
	})

	t.Run("adds tag to book by tag ID", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		book := &entities.Book{Title: "Test Book", Author: "Author"}
		require.NoError(t, db.SaveBook(book))

		tag, err := db.CreateTag("science", 0)
		require.NoError(t, err)

		controller := NewTagsController(db)
		router := gin.New()
		router.POST("/api/books/:id/tags", controller.AddTagToBook)

		body := bytes.NewBufferString(`{"tag_id": ` + string(rune(tag.ID+'0')) + `}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/books/1/tags", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestTagsController_RemoveTagFromBook(t *testing.T) {
	t.Run("removes tag from book", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		book := &entities.Book{Title: "Test Book", Author: "Author"}
		require.NoError(t, db.SaveBook(book))

		tag, err := db.CreateTag("remove-me", 0)
		require.NoError(t, err)
		require.NoError(t, db.AddTagToBook(1, tag.ID))

		controller := NewTagsController(db)
		router := gin.New()
		router.DELETE("/api/books/:id/tags/:tagId", controller.RemoveTagFromBook)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/books/1/tags/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify tag was removed
		updatedBook, err := db.GetBookByID(1)
		require.NoError(t, err)
		assert.Empty(t, updatedBook.Tags)
	})
}

func TestTagsController_AddTagToHighlight(t *testing.T) {
	t.Run("adds tag to highlight", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:      "Test Book",
			Author:     "Author",
			Highlights: []entities.Highlight{{Text: "Test highlight"}},
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewTagsController(db)
		router := gin.New()
		router.POST("/api/highlights/:id/tags", controller.AddTagToHighlight)

		body := bytes.NewBufferString(`{"tag_name": "important"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/highlights/1/tags", body)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify tag was added
		highlight, err := db.GetHighlightByID(1)
		require.NoError(t, err)
		assert.Len(t, highlight.Tags, 1)
		assert.Equal(t, "important", highlight.Tags[0].Name)
	})
}

func TestTagsController_GetBooksByTag(t *testing.T) {
	t.Run("returns books with specific tag", func(t *testing.T) {
		db, cleanup := setupTagsTestDB(t)
		defer cleanup()

		book1 := &entities.Book{Title: "Book 1", Author: "Author"}
		book2 := &entities.Book{Title: "Book 2", Author: "Author"}
		require.NoError(t, db.SaveBook(book1))
		require.NoError(t, db.SaveBook(book2))

		tag, err := db.CreateTag("fiction", 0)
		require.NoError(t, err)
		require.NoError(t, db.AddTagToBook(1, tag.ID))

		controller := NewTagsController(db)
		router := gin.New()
		router.GET("/api/tags/:id/books", controller.GetBooksByTag)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tags/1/books", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var books []entities.Book
		err = json.Unmarshal(w.Body.Bytes(), &books)
		require.NoError(t, err)
		assert.Len(t, books, 1)
		assert.Equal(t, "Book 1", books[0].Title)
	})
}
