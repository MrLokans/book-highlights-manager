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
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if err := fc.store.SetHighlightFavourite(id, true); err != nil {
		respondInternalError(c, err, "add favourite")
		return
	}

	highlight, err := fc.store.GetHighlightByID(id)
	if err != nil {
		respondSuccess(c, "favourite added")
		return
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "favourite-button", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "favourite added", "highlight": highlight})
}

// RemoveFavourite removes a highlight from favourites.
// DELETE /api/highlights/:id/favourite
func (fc *FavouritesController) RemoveFavourite(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if err := fc.store.SetHighlightFavourite(id, false); err != nil {
		respondInternalError(c, err, "remove favourite")
		return
	}

	highlight, err := fc.store.GetHighlightByID(id)
	if err != nil {
		respondSuccess(c, "favourite removed")
		return
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "favourite-button", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "favourite removed", "highlight": highlight})
}

// ListFavourites returns all favourite highlights with pagination.
// GET /api/highlights/favourites
func (fc *FavouritesController) ListFavourites(c *gin.Context) {
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

	highlights, total, err := fc.store.GetFavouriteHighlights(DefaultUserID, limit, offset)
	if err != nil {
		respondInternalError(c, err, "list favourites")
		return
	}

	data := gin.H{
		"Highlights": highlights,
		"Total":      total,
		"Limit":      limit,
		"Offset":     offset,
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "favourites-list", data)
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
	count, err := fc.store.GetFavouriteCount(DefaultUserID)
	if err != nil {
		respondInternalError(c, err, "get favourite count")
		return
	}

	respondHTMXOrJSON(c, http.StatusOK, "favourite-count", gin.H{"Count": count})
}

// FavouritesPage renders the favourites page.
// GET /favourites
func (fc *FavouritesController) FavouritesPage(c *gin.Context) {
	highlights, total, err := fc.store.GetFavouriteHighlights(DefaultUserID, 100, 0)
	if err != nil {
		respondInternalError(c, err, "load favourites page")
		return
	}

	c.HTML(http.StatusOK, "favourites", gin.H{
		"Highlights": highlights,
		"Total":      total,
		"Limit":      100,
		"Offset":     0,
		"Auth":       GetAuthTemplateData(c),
		"Demo":       GetDemoTemplateData(c),
		"Analytics":  GetAnalyticsTemplateData(c),
	})
}
