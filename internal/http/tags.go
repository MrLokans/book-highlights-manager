package http

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/tasks"
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
	store      TagStore
	taskClient *tasks.Client
}

func NewTagsController(store TagStore, taskClient *tasks.Client) *TagsController {
	return &TagsController{store: store, taskClient: taskClient}
}

// GetAllTags returns all tags for the current user
// GET /api/tags
func (tc *TagsController) GetAllTags(c *gin.Context) {
	tags, err := tc.store.GetTagsForUser(DefaultUserID)
	if err != nil {
		respondInternalError(c, err, "get all tags")
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
		respondBadRequest(c, "name is required")
		return
	}

	tag, err := tc.store.GetOrCreateTag(req.Name, DefaultUserID)
	if err != nil {
		respondInternalError(c, err, "create tag")
		return
	}

	respondCreated(c, tag)
}

// DeleteTag removes a tag
// DELETE /api/tags/:id
func (tc *TagsController) DeleteTag(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if err := tc.store.DeleteTag(id); err != nil {
		respondInternalError(c, err, "delete tag")
		return
	}

	if isHTMXRequest(c) {
		tags, _ := tc.store.GetTagsForUser(DefaultUserID)
		c.HTML(http.StatusOK, "tags-filter", gin.H{
			"Tags":          tags,
			"SelectedTagID": uint(0),
		})
		return
	}

	respondSuccess(c, "tag deleted")
}

// AddTagToBook adds a tag to a book
// POST /api/books/:id/tags
func (tc *TagsController) AddTagToBook(c *gin.Context) {
	bookID, ok := parseIDParam(c, "id")
	if !ok {
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
		tag, err := tc.store.GetOrCreateTag(req.TagName, DefaultUserID)
		if err != nil {
			respondInternalError(c, err, "get or create tag")
			return
		}
		tagID = tag.ID
	} else {
		respondBadRequest(c, "tag_id or tag_name required")
		return
	}

	if err := tc.store.AddTagToBook(bookID, tagID); err != nil {
		respondInternalError(c, err, "add tag to book")
		return
	}

	// Return updated book with tags for HTMX
	book, err := tc.store.GetBookByID(bookID)
	if err != nil {
		respondSuccess(c, "tag added")
		return
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "book-tags", gin.H{"Book": book})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag added", "tags": book.Tags})
}

// RemoveTagFromBook removes a tag from a book
// DELETE /api/books/:id/tags/:tagId
func (tc *TagsController) RemoveTagFromBook(c *gin.Context) {
	bookID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	tagID, ok := parseIDParam(c, "tagId")
	if !ok {
		return
	}

	if err := tc.store.RemoveTagFromBook(bookID, tagID); err != nil {
		respondInternalError(c, err, "remove tag from book")
		return
	}

	// Return updated book with tags for HTMX
	book, err := tc.store.GetBookByID(bookID)
	if err != nil {
		respondSuccess(c, "tag removed")
		return
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "book-tags", gin.H{"Book": book})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag removed", "tags": book.Tags})
}

// AddTagToHighlight adds a tag to a highlight
// POST /api/highlights/:id/tags
func (tc *TagsController) AddTagToHighlight(c *gin.Context) {
	highlightID, ok := parseIDParam(c, "id")
	if !ok {
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
		tag, err := tc.store.GetOrCreateTag(req.TagName, DefaultUserID)
		if err != nil {
			respondInternalError(c, err, "get or create tag")
			return
		}
		tagID = tag.ID
	} else {
		respondBadRequest(c, "tag_id or tag_name required")
		return
	}

	if err := tc.store.AddTagToHighlight(highlightID, tagID); err != nil {
		respondInternalError(c, err, "add tag to highlight")
		return
	}

	// Return updated highlight with tags for HTMX
	highlight, err := tc.store.GetHighlightByID(highlightID)
	if err != nil {
		respondSuccess(c, "tag added")
		return
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "highlight-tags", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag added", "tags": highlight.Tags})
}

// RemoveTagFromHighlight removes a tag from a highlight
// DELETE /api/highlights/:id/tags/:tagId
func (tc *TagsController) RemoveTagFromHighlight(c *gin.Context) {
	highlightID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	tagID, ok := parseIDParam(c, "tagId")
	if !ok {
		return
	}

	if err := tc.store.RemoveTagFromHighlight(highlightID, tagID); err != nil {
		respondInternalError(c, err, "remove tag from highlight")
		return
	}

	// Return updated highlight with tags for HTMX
	highlight, err := tc.store.GetHighlightByID(highlightID)
	if err != nil {
		respondSuccess(c, "tag removed")
		return
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "highlight-tags", highlight)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tag removed", "tags": highlight.Tags})
}

// GetBooksByTag returns all books with a specific tag
// GET /api/tags/:id/books
func (tc *TagsController) GetBooksByTag(c *gin.Context) {
	tagID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	books, err := tc.store.GetBooksByTag(tagID, DefaultUserID)
	if err != nil {
		respondInternalError(c, err, "get books by tag")
		return
	}

	respondHTMXOrJSON(c, http.StatusOK, "book-list", books)
}

// TagSuggest returns tag suggestions for autocomplete
// GET /api/tags/suggest?q=query
func (tc *TagsController) TagSuggest(c *gin.Context) {
	query := c.Query("q")

	// Require minimum 2 characters for autocomplete
	if len(query) < 2 {
		respondHTMXOrJSON(c, http.StatusOK, "tag-suggestions", []entities.Tag{})
		return
	}

	tags, err := tc.store.SearchTags(query, DefaultUserID)
	if err != nil {
		respondInternalError(c, err, "search tags")
		return
	}

	respondHTMXOrJSON(c, http.StatusOK, "tag-suggestions", tags)
}

// CleanupOrphanTags removes all tags that have no associated books or highlights.
// Requires the task queue to be enabled.
// POST /api/admin/tags/cleanup
func (tc *TagsController) CleanupOrphanTags(c *gin.Context) {
	if tc.taskClient == nil {
		if isHTMXRequest(c) {
			c.HTML(http.StatusOK, "tags-cleanup-result", gin.H{
				"Success": false,
				"Error":   "task queue is not enabled",
			})
			return
		}
		respondError(c, http.StatusServiceUnavailable, "task queue is not enabled")
		return
	}

	task := tasks.CleanupOrphanTagsTask{}
	ids, err := tc.taskClient.Add(task).Save()
	if err != nil {
		log.Printf("Failed to enqueue cleanup task: %v", err)
		if isHTMXRequest(c) {
			c.HTML(http.StatusOK, "tags-cleanup-result", gin.H{
				"Success": false,
				"Error":   "failed to start cleanup task",
			})
			return
		}
		respondInternalError(c, err, "enqueue cleanup task")
		return
	}
	log.Printf("Enqueued CleanupOrphanTagsTask with ID: %s", ids[0])

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "tags-cleanup-result", gin.H{
			"Success": true,
			"Message": "Cleanup task started",
		})
		return
	}

	respondAccepted(c, "cleanup task started", gin.H{"task_id": ids[0]})
}
