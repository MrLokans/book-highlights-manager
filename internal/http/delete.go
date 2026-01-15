package http

import (
	"net/http"

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
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if err := dc.store.DeleteBook(id); err != nil {
		respondInternalError(c, err, "delete book")
		return
	}

	respondHTMXOrJSON(c, http.StatusOK, "delete-success", gin.H{
		"Type":    "book",
		"Message": "Book deleted",
	})
}

// DeleteBookPermanently performs a hard delete on a book (cannot be restored, blocks re-import)
// DELETE /api/books/:id/permanent
func (dc *DeleteController) DeleteBookPermanently(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if err := dc.store.DeleteBookPermanently(id, DefaultUserID); err != nil {
		respondInternalError(c, err, "delete book permanently")
		return
	}

	respondHTMXOrJSON(c, http.StatusOK, "delete-success", gin.H{
		"Type":    "book",
		"Message": "Book permanently deleted",
	})
}

// DeleteHighlight performs a soft delete on a highlight
// DELETE /api/highlights/:id
func (dc *DeleteController) DeleteHighlight(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	// Get the highlight first to return the book ID for HTMX refresh
	highlight, _ := dc.store.GetHighlightByID(id)

	if err := dc.store.DeleteHighlight(id); err != nil {
		respondInternalError(c, err, "delete highlight")
		return
	}

	bookID := uint(0)
	if highlight != nil {
		bookID = highlight.BookID
	}
	respondHTMXOrJSON(c, http.StatusOK, "delete-success", gin.H{
		"Type":    "highlight",
		"Message": "Highlight deleted",
		"BookID":  bookID,
	})
}

// DeleteHighlightPermanently performs a hard delete on a highlight
// DELETE /api/highlights/:id/permanent
func (dc *DeleteController) DeleteHighlightPermanently(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	// Get the highlight first to return the book ID for HTMX refresh
	highlight, _ := dc.store.GetHighlightByID(id)

	if err := dc.store.DeleteHighlightPermanently(id, DefaultUserID); err != nil {
		respondInternalError(c, err, "delete highlight permanently")
		return
	}

	bookID := uint(0)
	if highlight != nil {
		bookID = highlight.BookID
	}
	respondHTMXOrJSON(c, http.StatusOK, "delete-success", gin.H{
		"Type":    "highlight",
		"Message": "Highlight permanently deleted",
		"BookID":  bookID,
	})
}
