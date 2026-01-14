package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
)

// FavouritesStore defines database operations for favourites management.
type FavouritesStore interface {
	SetHighlightFavourite(highlightID uint, isFavourite bool) error
	GetFavouriteHighlights(userID uint, limit, offset int) ([]entities.Highlight, int64, error)
	GetFavouriteHighlightsByBook(bookID uint) ([]entities.Highlight, error)
	GetFavouriteCount(userID uint) (int64, error)
	GetHighlightByID(id uint) (*entities.Highlight, error)
}

type FavouritesController struct {
	store FavouritesStore
}

func NewFavouritesController(store FavouritesStore) *FavouritesController {
	return &FavouritesController{store: store}
}

// AddFavourite marks a highlight as favourite.
// POST /api/highlights/:id/favourite
func (fc *FavouritesController) AddFavourite(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid highlight ID"})
		return
	}

	if err := fc.store.SetHighlightFavourite(uint(id), true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	highlight, err := fc.store.GetHighlightByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "favourite added"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "favourite-button", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "favourite added", "highlight": highlight})
}

// RemoveFavourite removes a highlight from favourites.
// DELETE /api/highlights/:id/favourite
func (fc *FavouritesController) RemoveFavourite(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid highlight ID"})
		return
	}

	if err := fc.store.SetHighlightFavourite(uint(id), false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	highlight, err := fc.store.GetHighlightByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "favourite removed"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "favourite-button", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "favourite removed", "highlight": highlight})
}

// ListFavourites returns all favourite highlights with pagination.
// GET /api/highlights/favourites
func (fc *FavouritesController) ListFavourites(c *gin.Context) {
	userID := uint(0) // Single-user mode

	limit := 50
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	highlights, total, err := fc.store.GetFavouriteHighlights(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "favourites-list", gin.H{
			"Highlights": highlights,
			"Total":      total,
			"Limit":      limit,
			"Offset":     offset,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"highlights": highlights,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// GetFavouriteCount returns the total count of favourites.
// GET /api/highlights/favourites/count
func (fc *FavouritesController) GetFavouriteCount(c *gin.Context) {
	userID := uint(0)

	count, err := fc.store.GetFavouriteCount(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "favourite-count", gin.H{"Count": count})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

// FavouritesPage renders the favourites page.
// GET /favourites
func (fc *FavouritesController) FavouritesPage(c *gin.Context) {
	userID := uint(0)

	highlights, total, err := fc.store.GetFavouriteHighlights(userID, 100, 0)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error loading favourites: %s", err.Error())
		return
	}

	c.HTML(http.StatusOK, "favourites", gin.H{
		"Highlights": highlights,
		"Total":      total,
		"Limit":      100,
		"Offset":     0,
	})
}
