package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
)

// DeleteStore defines database operations for entity deletion.
type DeleteStore interface {
	GetBookByID(id uint) (*entities.Book, error)
	GetHighlightByID(id uint) (*entities.Highlight, error)
	DeleteBook(id uint) error
	DeleteBookPermanently(id uint, userID uint) error
	DeleteHighlight(id uint) error
	DeleteHighlightPermanently(id uint, userID uint) error
}

type DeleteController struct {
	store DeleteStore
}

func NewDeleteController(store DeleteStore) *DeleteController {
	return &DeleteController{store: store}
}

// DeleteBook performs a soft delete on a book (can be restored)
// DELETE /api/books/:id
func (dc *DeleteController) DeleteBook(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book ID"})
		return
	}

	if err := dc.store.DeleteBook(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "delete-success", gin.H{
			"Type":    "book",
			"Message": "Book deleted",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "book deleted"})
}

// DeleteBookPermanently performs a hard delete on a book (cannot be restored, blocks re-import)
// DELETE /api/books/:id/permanent
func (dc *DeleteController) DeleteBookPermanently(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book ID"})
		return
	}

	// For now we use userID 0 for the default user (single-user mode)
	userID := uint(0)

	if err := dc.store.DeleteBookPermanently(uint(id), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "delete-success", gin.H{
			"Type":    "book",
			"Message": "Book permanently deleted",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "book permanently deleted"})
}

// DeleteHighlight performs a soft delete on a highlight
// DELETE /api/highlights/:id
func (dc *DeleteController) DeleteHighlight(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid highlight ID"})
		return
	}

	// Get the highlight first to return the book ID for HTMX refresh
	highlight, _ := dc.store.GetHighlightByID(uint(id))

	if err := dc.store.DeleteHighlight(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		bookID := uint(0)
		if highlight != nil {
			bookID = highlight.BookID
		}
		c.HTML(http.StatusOK, "delete-success", gin.H{
			"Type":    "highlight",
			"Message": "Highlight deleted",
			"BookID":  bookID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "highlight deleted"})
}

// DeleteHighlightPermanently performs a hard delete on a highlight
// DELETE /api/highlights/:id/permanent
func (dc *DeleteController) DeleteHighlightPermanently(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid highlight ID"})
		return
	}

	// For now we use userID 0 for the default user (single-user mode)
	userID := uint(0)

	// Get the highlight first to return the book ID for HTMX refresh
	highlight, _ := dc.store.GetHighlightByID(uint(id))

	if err := dc.store.DeleteHighlightPermanently(uint(id), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		bookID := uint(0)
		if highlight != nil {
			bookID = highlight.BookID
		}
		c.HTML(http.StatusOK, "delete-success", gin.H{
			"Type":    "highlight",
			"Message": "Highlight permanently deleted",
			"BookID":  bookID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "highlight permanently deleted"})
}
