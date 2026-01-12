package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/covers"
	"github.com/mrlokans/assistant/internal/exporters"
)

// CoversController handles book cover requests.
type CoversController struct {
	cache      *covers.Cache
	bookReader exporters.BookReader
}

// NewCoversController creates a new CoversController.
func NewCoversController(cache *covers.Cache, reader exporters.BookReader) *CoversController {
	return &CoversController{
		cache:      cache,
		bookReader: reader,
	}
}

// GetCover serves a cached book cover image.
// GET /api/books/:id/cover
func (cc *CoversController) GetCover(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	book, err := cc.bookReader.GetBookByID(uint(id))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	if book.CoverURL == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// Get cached cover (will fetch if not cached)
	cachePath, err := cc.cache.GetCover(uint(id), book.CoverURL)
	if err != nil || cachePath == "" {
		// Fallback: redirect to original URL
		c.Redirect(http.StatusTemporaryRedirect, book.CoverURL)
		return
	}

	// Serve the cached file
	c.File(cachePath)
}
