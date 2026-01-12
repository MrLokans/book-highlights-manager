package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
)

// TagStore defines database operations for tag management.
type TagStore interface {
	CreateTag(name string, userID uint) (*entities.Tag, error)
	GetOrCreateTag(name string, userID uint) (*entities.Tag, error)
	GetTagsForUser(userID uint) ([]entities.Tag, error)
	SearchTags(query string, userID uint) ([]entities.Tag, error)
	GetTagByID(id uint) (*entities.Tag, error)
	DeleteTag(id uint) error
	DeleteOrphanTags() (int64, error)
	AddTagToBook(bookID, tagID uint) error
	RemoveTagFromBook(bookID, tagID uint) error
	AddTagToHighlight(highlightID, tagID uint) error
	RemoveTagFromHighlight(highlightID, tagID uint) error
	GetBooksByTag(tagID uint, userID uint) ([]entities.Book, error)
	GetBookByID(id uint) (*entities.Book, error)
	GetHighlightByID(id uint) (*entities.Highlight, error)
}

type TagsController struct {
	store TagStore
}

func NewTagsController(store TagStore) *TagsController {
	return &TagsController{store: store}
}

// GetAllTags returns all tags for the current user
// GET /api/tags
func (tc *TagsController) GetAllTags(c *gin.Context) {
	// For now we use userID 0 for the default user (single-user mode)
	userID := uint(0)
	tags, err := tc.store.GetTagsForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// CreateTag creates a new tag
// POST /api/tags
func (tc *TagsController) CreateTag(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	userID := uint(0)
	tag, err := tc.store.GetOrCreateTag(req.Name, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

// DeleteTag removes a tag
// DELETE /api/tags/:id
func (tc *TagsController) DeleteTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tag ID"})
		return
	}

	if err := tc.store.DeleteTag(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		userID := uint(0)
		tags, _ := tc.store.GetTagsForUser(userID)
		c.HTML(http.StatusOK, "tags-filter", gin.H{
			"Tags":          tags,
			"SelectedTagID": uint(0),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag deleted"})
}

// AddTagToBook adds a tag to a book
// POST /api/books/:id/tags
func (tc *TagsController) AddTagToBook(c *gin.Context) {
	bookIDStr := c.Param("id")
	bookID, err := strconv.ParseUint(bookIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book ID"})
		return
	}

	var req struct {
		TagID   uint   `json:"tag_id" form:"tag_id"`
		TagName string `json:"tag_name" form:"tag_name"`
		Q       string `json:"q" form:"q"` // alias for tag_name (used by autocomplete input)
	}
	_ = c.ShouldBind(&req)

	// Use q as fallback for tag_name (autocomplete input uses q)
	if req.TagName == "" && req.Q != "" {
		req.TagName = req.Q
	}

	var tagID uint
	if req.TagID > 0 {
		tagID = req.TagID
	} else if req.TagName != "" {
		tag, err := tc.store.GetOrCreateTag(req.TagName, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		tagID = tag.ID
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_id or tag_name required"})
		return
	}

	if err := tc.store.AddTagToBook(uint(bookID), tagID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return updated book with tags for HTMX
	book, err := tc.store.GetBookByID(uint(bookID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "tag added"})
		return
	}

	// Check Accept header for HTMX
	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "book-tags", gin.H{"Book": book})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag added", "tags": book.Tags})
}

// RemoveTagFromBook removes a tag from a book
// DELETE /api/books/:id/tags/:tagId
func (tc *TagsController) RemoveTagFromBook(c *gin.Context) {
	bookIDStr := c.Param("id")
	bookID, err := strconv.ParseUint(bookIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book ID"})
		return
	}

	tagIDStr := c.Param("tagId")
	tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tag ID"})
		return
	}

	if err := tc.store.RemoveTagFromBook(uint(bookID), uint(tagID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return updated book with tags for HTMX
	book, err := tc.store.GetBookByID(uint(bookID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "tag removed"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "book-tags", gin.H{"Book": book})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag removed", "tags": book.Tags})
}

// AddTagToHighlight adds a tag to a highlight
// POST /api/highlights/:id/tags
func (tc *TagsController) AddTagToHighlight(c *gin.Context) {
	highlightIDStr := c.Param("id")
	highlightID, err := strconv.ParseUint(highlightIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid highlight ID"})
		return
	}

	var req struct {
		TagID   uint   `json:"tag_id" form:"tag_id"`
		TagName string `json:"tag_name" form:"tag_name"`
		Q       string `json:"q" form:"q"` // alias for tag_name (used by autocomplete input)
	}
	_ = c.ShouldBind(&req)

	// Use q as fallback for tag_name (autocomplete input uses q)
	if req.TagName == "" && req.Q != "" {
		req.TagName = req.Q
	}

	var tagID uint
	if req.TagID > 0 {
		tagID = req.TagID
	} else if req.TagName != "" {
		tag, err := tc.store.GetOrCreateTag(req.TagName, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		tagID = tag.ID
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_id or tag_name required"})
		return
	}

	if err := tc.store.AddTagToHighlight(uint(highlightID), tagID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return updated highlight with tags for HTMX
	highlight, err := tc.store.GetHighlightByID(uint(highlightID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "tag added"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "highlight-tags", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag added", "tags": highlight.Tags})
}

// RemoveTagFromHighlight removes a tag from a highlight
// DELETE /api/highlights/:id/tags/:tagId
func (tc *TagsController) RemoveTagFromHighlight(c *gin.Context) {
	highlightIDStr := c.Param("id")
	highlightID, err := strconv.ParseUint(highlightIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid highlight ID"})
		return
	}

	tagIDStr := c.Param("tagId")
	tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tag ID"})
		return
	}

	if err := tc.store.RemoveTagFromHighlight(uint(highlightID), uint(tagID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return updated highlight with tags for HTMX
	highlight, err := tc.store.GetHighlightByID(uint(highlightID))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "tag removed"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "highlight-tags", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag removed", "tags": highlight.Tags})
}

// GetBooksByTag returns all books with a specific tag
// GET /api/tags/:id/books
func (tc *TagsController) GetBooksByTag(c *gin.Context) {
	tagIDStr := c.Param("id")
	tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tag ID"})
		return
	}

	userID := uint(0)
	books, err := tc.store.GetBooksByTag(uint(tagID), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "book-list", books)
		return
	}

	c.JSON(http.StatusOK, books)
}

// TagSuggest returns tag suggestions for autocomplete
// GET /api/tags/suggest?q=query
func (tc *TagsController) TagSuggest(c *gin.Context) {
	query := c.Query("q")
	userID := uint(0)

	// Require minimum 2 characters for autocomplete
	if len(query) < 2 {
		if c.GetHeader("HX-Request") == "true" {
			c.HTML(http.StatusOK, "tag-suggestions", []entities.Tag{})
			return
		}
		c.JSON(http.StatusOK, []entities.Tag{})
		return
	}

	tags, err := tc.store.SearchTags(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "tag-suggestions", tags)
		return
	}

	c.JSON(http.StatusOK, tags)
}

// CleanupOrphanTags removes all tags that have no associated books or highlights
// POST /api/admin/tags/cleanup
func (tc *TagsController) CleanupOrphanTags(c *gin.Context) {
	deleted, err := tc.store.DeleteOrphanTags()
	if err != nil {
		if c.GetHeader("HX-Request") == "true" {
			c.HTML(http.StatusOK, "tags-cleanup-result", gin.H{
				"Success": false,
				"Error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "tags-cleanup-result", gin.H{
			"Success": true,
			"Deleted": deleted,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "orphan tags cleaned up",
		"deleted": deleted,
	})
}

