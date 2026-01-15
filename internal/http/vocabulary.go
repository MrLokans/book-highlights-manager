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
		words, total, err = vc.store.GetWordsByStatus(DefaultUserID, status, limit, offset)
	} else {
		words, total, err = vc.store.GetAllWords(DefaultUserID, limit, offset)
	}

	if err != nil {
		respondInternalError(c, err, "list words")
		return
	}

	data := gin.H{
		"Words":  words,
		"Total":  total,
		"Limit":  limit,
		"Offset": offset,
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "vocabulary-list", data)
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
	words, _, err := vc.store.GetAllWords(DefaultUserID, 0, 0)
	if err != nil {
		respondInternalError(c, err, "get words list")
		return
	}

	// Return minimal data
	type wordItem struct {
		ID     uint                `json:"id"`
		Word   string              `json:"word"`
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
		respondBadRequest(c, err.Error())
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
		respondError(c, http.StatusConflict, "word already exists")
		return
	}

	if err := vc.store.AddWord(word); err != nil {
		respondInternalError(c, err, "add word")
		return
	}

	// Auto-enrich if requested and task queue available
	if req.AutoEnrich && vc.taskClient != nil {
		_, _ = vc.taskClient.Add(tasks.EnrichWordTask{WordID: word.ID}).Save()
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusCreated, "word-card", word)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"word": word})
}

// GetWord returns a word with all definitions.
// GET /api/vocabulary/:id
func (vc *VocabularyController) GetWord(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	word, err := vc.store.GetWordByID(id)
	if err != nil {
		respondNotFound(c, "word")
		return
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "word-detail", word)
		return
	}

	c.JSON(http.StatusOK, gin.H{"word": word})
}

// UpdateWord updates a word's fields.
// PATCH /api/vocabulary/:id
func (vc *VocabularyController) UpdateWord(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	word, err := vc.store.GetWordByID(id)
	if err != nil {
		respondNotFound(c, "word")
		return
	}

	var updates struct {
		Word    *string `json:"word,omitempty"`
		Context *string `json:"context,omitempty"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		respondBadRequest(c, err.Error())
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
		respondInternalError(c, err, "update word")
		return
	}

	// Auto-enrich if word changed and task queue available
	if wordTextChanged && vc.taskClient != nil {
		_, _ = vc.taskClient.Add(tasks.EnrichWordTask{WordID: word.ID}).Save()
	}

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "word-card", word)
		return
	}

	c.JSON(http.StatusOK, gin.H{"word": word})
}

// DeleteWord removes a word.
// DELETE /api/vocabulary/:id
func (vc *VocabularyController) DeleteWord(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	if err := vc.store.DeleteWord(id); err != nil {
		respondInternalError(c, err, "delete word")
		return
	}

	if isHTMXRequest(c) {
		c.String(http.StatusOK, "")
		return
	}

	respondSuccess(c, "word deleted")
}

// EnrichWord triggers enrichment for a single word.
// POST /api/vocabulary/:id/enrich
func (vc *VocabularyController) EnrichWord(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	word, err := vc.store.GetWordByID(id)
	if err != nil {
		respondNotFound(c, "word")
		return
	}

	// Use task queue if available
	if vc.taskClient != nil {
		if _, err := vc.taskClient.Add(tasks.EnrichWordTask{WordID: id}).Save(); err != nil {
			respondInternalError(c, err, "queue enrichment task")
			return
		}

		if isHTMXRequest(c) {
			// Return the word with pending status indicator
			word.Status = entities.WordStatusPending
			c.HTML(http.StatusAccepted, "word-card", word)
			return
		}

		respondAccepted(c, "enrichment task queued", gin.H{"word_id": id})
		return
	}

	// Synchronous enrichment if no task queue
	result, err := vc.dictClient.Lookup(c.Request.Context(), word.Word)
	if err != nil {
		_ = vc.store.UpdateWordStatus(id, entities.WordStatusFailed, err.Error())
		respondInternalError(c, err, "dictionary lookup")
		return
	}

	if err := vc.store.SaveDefinitions(id, result.Definitions); err != nil {
		respondInternalError(c, err, "save definitions")
		return
	}

	_ = vc.store.UpdateWordStatus(id, entities.WordStatusEnriched, "")

	updatedWord, _ := vc.store.GetWordByID(id)

	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "word-card", updatedWord)
		return
	}

	c.JSON(http.StatusOK, gin.H{"word": updatedWord})
}

// EnrichAllWords triggers enrichment for all pending words.
// POST /api/vocabulary/enrich-all
func (vc *VocabularyController) EnrichAllWords(c *gin.Context) {
	if vc.taskClient == nil {
		respondError(c, http.StatusServiceUnavailable, "task queue not available")
		return
	}

	if _, err := vc.taskClient.Add(tasks.EnrichAllPendingWordsTask{}).Save(); err != nil {
		respondInternalError(c, err, "queue batch enrichment task")
		return
	}

	respondAccepted(c, "batch enrichment task queued", nil)
}

// GetWordsByHighlight returns words for a specific highlight.
// GET /api/highlights/:id/vocabulary
func (vc *VocabularyController) GetWordsByHighlight(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	words, err := vc.store.GetWordsByHighlight(id)
	if err != nil {
		respondInternalError(c, err, "get words by highlight")
		return
	}

	c.JSON(http.StatusOK, gin.H{"words": words})
}

// GetVocabularyStats returns vocabulary statistics.
// GET /api/vocabulary/stats
func (vc *VocabularyController) GetVocabularyStats(c *gin.Context) {
	total, pending, enriched, failed, err := vc.store.GetVocabularyStats(DefaultUserID)
	if err != nil {
		respondInternalError(c, err, "get vocabulary stats")
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
		respondBadRequest(c, "query parameter 'q' is required")
		return
	}

	limit := 20

	words, err := vc.store.SearchWords(query, DefaultUserID, limit)
	if err != nil {
		respondInternalError(c, err, "search words")
		return
	}

	if isHTMXRequest(c) {
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
	words, total, err := vc.store.GetAllWords(DefaultUserID, 100, 0)
	if err != nil {
		respondInternalError(c, err, "load vocabulary page")
		return
	}

	_, pending, enriched, failed, _ := vc.store.GetVocabularyStats(DefaultUserID)

	c.HTML(http.StatusOK, "vocabulary", gin.H{
		"Words":    words,
		"Total":    total,
		"Pending":  pending,
		"Enriched": enriched,
		"Failed":   failed,
		"Auth":     GetAuthTemplateData(c),
		"Demo":     GetDemoTemplateData(c),
	})
}
