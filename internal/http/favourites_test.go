package http

import (
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

func setupFavouritesTestDB(t *testing.T) (*database.Database, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbPath := "./test_favourites_" + strings.ReplaceAll(t.Name(), "/", "_") + ".db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, cleanup
}

func TestFavouritesController_AddFavourite(t *testing.T) {
	t.Run("marks highlight as favourite", func(t *testing.T) {
		db, cleanup := setupFavouritesTestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:      "Test Book",
			Author:     "Author",
			Highlights: []entities.Highlight{{Text: "Test highlight"}},
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewFavouritesController(db)
		router := gin.New()
		router.POST("/api/highlights/:id/favourite", controller.AddFavourite)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/highlights/1/favourite", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify highlight is now a favourite
		highlight, err := db.GetHighlightByID(1)
		require.NoError(t, err)
		assert.True(t, highlight.IsFavorite)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		db, cleanup := setupFavouritesTestDB(t)
		defer cleanup()

		controller := NewFavouritesController(db)
		router := gin.New()
		router.POST("/api/highlights/:id/favourite", controller.AddFavourite)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/highlights/invalid/favourite", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestFavouritesController_RemoveFavourite(t *testing.T) {
	t.Run("removes highlight from favourites", func(t *testing.T) {
		db, cleanup := setupFavouritesTestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:      "Test Book",
			Author:     "Author",
			Highlights: []entities.Highlight{{Text: "Test highlight", IsFavorite: true}},
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewFavouritesController(db)
		router := gin.New()
		router.DELETE("/api/highlights/:id/favourite", controller.RemoveFavourite)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/highlights/1/favourite", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify highlight is no longer a favourite
		highlight, err := db.GetHighlightByID(1)
		require.NoError(t, err)
		assert.False(t, highlight.IsFavorite)
	})
}

func TestFavouritesController_ListFavourites(t *testing.T) {
	t.Run("returns empty list when no favourites", func(t *testing.T) {
		db, cleanup := setupFavouritesTestDB(t)
		defer cleanup()

		controller := NewFavouritesController(db)
		router := gin.New()
		router.GET("/api/highlights/favourites", controller.ListFavourites)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/highlights/favourites", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Highlights []entities.Highlight `json:"highlights"`
			Total      int64                `json:"total"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Empty(t, response.Highlights)
		assert.Equal(t, int64(0), response.Total)
	})

	t.Run("returns only favourite highlights", func(t *testing.T) {
		db, cleanup := setupFavouritesTestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:  "Test Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "Favourite highlight", IsFavorite: true},
				{Text: "Normal highlight", IsFavorite: false},
				{Text: "Another favourite", IsFavorite: true},
			},
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewFavouritesController(db)
		router := gin.New()
		router.GET("/api/highlights/favourites", controller.ListFavourites)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/highlights/favourites", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Highlights []entities.Highlight `json:"highlights"`
			Total      int64                `json:"total"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response.Highlights, 2)
		assert.Equal(t, int64(2), response.Total)
	})

	t.Run("supports pagination", func(t *testing.T) {
		db, cleanup := setupFavouritesTestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:  "Test Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "Favourite 1", IsFavorite: true},
				{Text: "Favourite 2", IsFavorite: true},
				{Text: "Favourite 3", IsFavorite: true},
			},
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewFavouritesController(db)
		router := gin.New()
		router.GET("/api/highlights/favourites", controller.ListFavourites)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/highlights/favourites?limit=2&offset=0", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Highlights []entities.Highlight `json:"highlights"`
			Total      int64                `json:"total"`
			Limit      int                  `json:"limit"`
			Offset     int                  `json:"offset"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response.Highlights, 2)
		assert.Equal(t, int64(3), response.Total)
		assert.Equal(t, 2, response.Limit)
		assert.Equal(t, 0, response.Offset)
	})
}

func TestFavouritesController_GetFavouriteCount(t *testing.T) {
	t.Run("returns correct count", func(t *testing.T) {
		db, cleanup := setupFavouritesTestDB(t)
		defer cleanup()

		book := &entities.Book{
			Title:  "Test Book",
			Author: "Author",
			Highlights: []entities.Highlight{
				{Text: "Favourite 1", IsFavorite: true},
				{Text: "Normal", IsFavorite: false},
				{Text: "Favourite 2", IsFavorite: true},
			},
		}
		require.NoError(t, db.SaveBook(book))

		controller := NewFavouritesController(db)
		router := gin.New()
		router.GET("/api/highlights/favourites/count", controller.GetFavouriteCount)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/highlights/favourites/count", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Count int64 `json:"count"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, int64(2), response.Count)
	})
}
