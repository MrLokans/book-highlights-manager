package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/dictionary"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/tasks"
)

// VocabularyStore defines database operations for vocabulary management.
type VocabularyStore interface {
	AddWord(word *entities.Word) error
	GetAllWords(userID uint, limit, offset int) ([]entities.Word, int64, error)
	GetWordByID(id uint) (*entities.Word, error)
	UpdateWord(word *entities.Word) error
	DeleteWord(id uint) error
	GetPendingWords(limit int) ([]entities.Word, error)
	SaveDefinitions(wordID uint, definitions []entities.WordDefinition) error
	UpdateWordStatus(id uint, status entities.WordStatus, errorMsg string) error
	GetWordsByHighlight(highlightID uint) ([]entities.Word, error)
	GetWordsByBook(bookID uint) ([]entities.Word, error)
	FindWordBySource(word, sourceBookTitle, sourceBookAuthor, sourceHighlightText string, userID uint) (*entities.Word, error)
	SearchWords(query string, userID uint, limit int) ([]entities.Word, error)
	GetVocabularyStats(userID uint) (total, pending, enriched, failed int64, err error)
	GetWordsByStatus(userID uint, status entities.WordStatus, limit, offset int) ([]entities.Word, int64, error)
	GetHighlightByID(id uint) (*entities.Highlight, error)
	GetBookByID(id uint) (*entities.Book, error)
}

type VocabularyController struct {
	store      VocabularyStore
	dictClient dictionary.Client
	taskClient *tasks.Client
}

func NewVocabularyController(store VocabularyStore, dictClient dictionary.Client, taskClient *tasks.Client) *VocabularyController {
	return &VocabularyController{
		store:      store,
		dictClient: dictClient,
		taskClient: taskClient,
	}
}

// AddWordRequest is the request body for adding a word.
type AddWordRequest struct {
	Word        string `json:"word" binding:"required"`
	HighlightID *uint  `json:"highlight_id,omitempty"`
	BookID      *uint  `json:"book_id,omitempty"`
	Context     string `json:"context,omitempty"`
	AutoEnrich  bool   `json:"auto_enrich,omitempty"`
}

// ListWords returns paginated vocabulary list.
// GET /api/vocabulary
func (vc *VocabularyController) ListWords(c *gin.Context) {
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

	// Filter by status if provided
	statusFilter := c.Query("status")

	var words []entities.Word
	var total int64
	var err error

	if statusFilter != "" {
		status := entities.WordStatus(statusFilter)
		words, total, err = vc.store.GetWordsByStatus(userID, status, limit, offset)
	} else {
		words, total, err = vc.store.GetAllWords(userID, limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "vocabulary-list", gin.H{
			"Words":  words,
			"Total":  total,
			"Limit":  limit,
			"Offset": offset,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"words":  words,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetWordsList returns lightweight word list (word + status only).
// GET /api/vocabulary/words
func (vc *VocabularyController) GetWordsList(c *gin.Context) {
	userID := uint(0)

	words, _, err := vc.store.GetAllWords(userID, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return minimal data
	type wordItem struct {
		ID     uint               `json:"id"`
		Word   string             `json:"word"`
		Status entities.WordStatus `json:"status"`
	}

	items := make([]wordItem, len(words))
	for i, w := range words {
		items[i] = wordItem{ID: w.ID, Word: w.Word, Status: w.Status}
	}

	c.JSON(http.StatusOK, gin.H{"words": items})
}

// AddWord creates a new vocabulary word.
// POST /api/vocabulary
func (vc *VocabularyController) AddWord(c *gin.Context) {
	var req AddWordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	word := &entities.Word{
		Word:    req.Word,
		Context: req.Context,
		Status:  entities.WordStatusPending,
	}

	// Link to highlight/book and denormalize source info
	if req.HighlightID != nil {
		highlight, err := vc.store.GetHighlightByID(*req.HighlightID)
		if err == nil {
			word.HighlightID = req.HighlightID
			word.SourceHighlightText = highlight.Text

			book, _ := vc.store.GetBookByID(highlight.BookID)
			if book != nil {
				word.BookID = &book.ID
				word.SourceBookTitle = book.Title
				word.SourceBookAuthor = book.Author
			}
		}
	} else if req.BookID != nil {
		book, err := vc.store.GetBookByID(*req.BookID)
		if err == nil {
			word.BookID = req.BookID
			word.SourceBookTitle = book.Title
			word.SourceBookAuthor = book.Author
		}
	}

	// Check for duplicate
	existing, _ := vc.store.FindWordBySource(word.Word, word.SourceBookTitle, word.SourceBookAuthor, word.SourceHighlightText, word.UserID)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "word already exists", "word": existing})
		return
	}

	if err := vc.store.AddWord(word); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Auto-enrich if requested and task queue available
	if req.AutoEnrich && vc.taskClient != nil {
		_, _ = vc.taskClient.Add(tasks.EnrichWordTask{WordID: word.ID}).Save()
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusCreated, "word-card", word)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"word": word})
}

// GetWord returns a word with all definitions.
// GET /api/vocabulary/:id
func (vc *VocabularyController) GetWord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid word ID"})
		return
	}

	word, err := vc.store.GetWordByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "word not found"})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "word-detail", word)
		return
	}

	c.JSON(http.StatusOK, gin.H{"word": word})
}

// UpdateWord updates a word's fields.
// PATCH /api/vocabulary/:id
func (vc *VocabularyController) UpdateWord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid word ID"})
		return
	}

	word, err := vc.store.GetWordByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "word not found"})
		return
	}

	var updates struct {
		Word    *string `json:"word,omitempty"`
		Context *string `json:"context,omitempty"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wordTextChanged := false
	if updates.Word != nil && *updates.Word != word.Word {
		word.Word = *updates.Word
		wordTextChanged = true
	}
	if updates.Context != nil {
		word.Context = *updates.Context
	}

	// Reset to pending if word text changed to trigger re-enrichment
	if wordTextChanged {
		word.Status = entities.WordStatusPending
		word.EnrichmentError = ""
	}

	if err := vc.store.UpdateWord(word); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Auto-enrich if word changed and task queue available
	if wordTextChanged && vc.taskClient != nil {
		_, _ = vc.taskClient.Add(tasks.EnrichWordTask{WordID: word.ID}).Save()
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "word-card", word)
		return
	}

	c.JSON(http.StatusOK, gin.H{"word": word})
}

// DeleteWord removes a word.
// DELETE /api/vocabulary/:id
func (vc *VocabularyController) DeleteWord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid word ID"})
		return
	}

	if err := vc.store.DeleteWord(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.String(http.StatusOK, "")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "word deleted"})
}

// EnrichWord triggers enrichment for a single word.
// POST /api/vocabulary/:id/enrich
func (vc *VocabularyController) EnrichWord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid word ID"})
		return
	}

	word, err := vc.store.GetWordByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "word not found"})
		return
	}

	// Use task queue if available
	if vc.taskClient != nil {
		if _, err := vc.taskClient.Add(tasks.EnrichWordTask{WordID: uint(id)}).Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue enrichment task"})
			return
		}

		if c.GetHeader("HX-Request") == "true" {
			// Return the word with pending status indicator
			word.Status = entities.WordStatusPending
			c.HTML(http.StatusAccepted, "word-card", word)
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"message": "enrichment task queued", "word_id": id})
		return
	}

	// Synchronous enrichment if no task queue
	result, err := vc.dictClient.Lookup(c.Request.Context(), word.Word)
	if err != nil {
		_ = vc.store.UpdateWordStatus(uint(id), entities.WordStatusFailed, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := vc.store.SaveDefinitions(uint(id), result.Definitions); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_ = vc.store.UpdateWordStatus(uint(id), entities.WordStatusEnriched, "")

	updatedWord, _ := vc.store.GetWordByID(uint(id))

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "word-card", updatedWord)
		return
	}

	c.JSON(http.StatusOK, gin.H{"word": updatedWord})
}

// EnrichAllWords triggers enrichment for all pending words.
// POST /api/vocabulary/enrich-all
func (vc *VocabularyController) EnrichAllWords(c *gin.Context) {
	if vc.taskClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not available"})
		return
	}

	if _, err := vc.taskClient.Add(tasks.EnrichAllPendingWordsTask{}).Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue enrichment task"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"message": "batch enrichment task queued"})
}

// GetWordsByHighlight returns words for a specific highlight.
// GET /api/highlights/:id/vocabulary
func (vc *VocabularyController) GetWordsByHighlight(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid highlight ID"})
		return
	}

	words, err := vc.store.GetWordsByHighlight(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"words": words})
}

// GetVocabularyStats returns vocabulary statistics.
// GET /api/vocabulary/stats
func (vc *VocabularyController) GetVocabularyStats(c *gin.Context) {
	userID := uint(0)

	total, pending, enriched, failed, err := vc.store.GetVocabularyStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":    total,
		"pending":  pending,
		"enriched": enriched,
		"failed":   failed,
	})
}

// SearchWords searches vocabulary words.
// GET /api/vocabulary/search
func (vc *VocabularyController) SearchWords(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	userID := uint(0)
	limit := 20

	words, err := vc.store.SearchWords(query, userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if c.GetHeader("HX-Request") == "true" {
		c.HTML(http.StatusOK, "vocabulary-list", gin.H{
			"Words": words,
			"Total": int64(len(words)),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"words": words})
}

// VocabularyPage renders the vocabulary management page.
// GET /vocabulary
func (vc *VocabularyController) VocabularyPage(c *gin.Context) {
	userID := uint(0)

	words, total, err := vc.store.GetAllWords(userID, 100, 0)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error loading vocabulary: %s", err.Error())
		return
	}

	_, pending, enriched, failed, _ := vc.store.GetVocabularyStats(userID)

	c.HTML(http.StatusOK, "vocabulary", gin.H{
		"Words":    words,
		"Total":    total,
		"Pending":  pending,
		"Enriched": enriched,
		"Failed":   failed,
	})
}
