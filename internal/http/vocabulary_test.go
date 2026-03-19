package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockVocabStore struct {
	words      []entities.Word
	total      int64
	word       *entities.Word
	highlight  *entities.Highlight
	book       *entities.Book
	addErr     error
	getErr     error
	updateErr  error
	deleteErr  error
	statsTotal int64
	statsPend  int64
	statsEnr   int64
	statsFail  int64
}

func (m *mockVocabStore) AddWord(_ *entities.Word) error { return m.addErr }
func (m *mockVocabStore) GetAllWords(_ uint, _, _ int) ([]entities.Word, int64, error) {
	return m.words, m.total, m.getErr
}
func (m *mockVocabStore) GetWordByID(_ uint) (*entities.Word, error) {
	if m.word == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return m.word, nil
}
func (m *mockVocabStore) UpdateWord(_ *entities.Word) error                         { return m.updateErr }
func (m *mockVocabStore) DeleteWord(_ uint) error                                   { return m.deleteErr }
func (m *mockVocabStore) GetPendingWords(_ int) ([]entities.Word, error)            { return nil, nil }
func (m *mockVocabStore) SaveDefinitions(_ uint, _ []entities.WordDefinition) error { return nil }
func (m *mockVocabStore) UpdateWordStatus(_ uint, _ entities.WordStatus, _ string) error {
	return nil
}
func (m *mockVocabStore) GetWordsByHighlight(_ uint) ([]entities.Word, error) { return m.words, nil }
func (m *mockVocabStore) GetWordsByBook(_ uint) ([]entities.Word, error)      { return m.words, nil }
func (m *mockVocabStore) FindWordBySource(_, _, _, _ string, _ uint) (*entities.Word, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockVocabStore) SearchWords(_ string, _ uint, _ int) ([]entities.Word, error) {
	return m.words, nil
}
func (m *mockVocabStore) GetVocabularyStats(_ uint) (int64, int64, int64, int64, error) {
	return m.statsTotal, m.statsPend, m.statsEnr, m.statsFail, nil
}
func (m *mockVocabStore) GetWordsByStatus(_ uint, _ entities.WordStatus, _, _ int) ([]entities.Word, int64, error) {
	return m.words, m.total, nil
}
func (m *mockVocabStore) GetHighlightByID(_ uint) (*entities.Highlight, error) {
	if m.highlight == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return m.highlight, nil
}
func (m *mockVocabStore) GetBookByID(_ uint) (*entities.Book, error) {
	if m.book == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return m.book, nil
}

func TestVocabularyController_ListWords_JSON(t *testing.T) {
	store := &mockVocabStore{
		words: []entities.Word{
			{Word: "ephemeral", Status: entities.WordStatusEnriched},
			{Word: "cogent", Status: entities.WordStatusPending},
		},
		total: 2,
	}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary", controller.ListWords)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(2), resp["total"])
	words := resp["words"].([]any)
	assert.Len(t, words, 2)
}

func TestVocabularyController_ListWords_HTMX(t *testing.T) {
	store := &mockVocabStore{words: []entities.Word{{Word: "test"}}, total: 1}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary", controller.ListWords)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary", nil)
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:vocabulary-list")
}

func TestVocabularyController_GetWordsList(t *testing.T) {
	store := &mockVocabStore{
		words: []entities.Word{
			{Word: "test", Status: entities.WordStatusPending},
		},
		total: 1,
	}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary/words", controller.GetWordsList)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary/words", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
}

func TestVocabularyController_AddWord(t *testing.T) {
	t.Run("adds word successfully", func(t *testing.T) {
		store := &mockVocabStore{}
		controller := NewVocabularyController(store, nil, nil)

		router := newTestRouter(t)
		router.POST("/api/vocabulary", controller.AddWord)

		body, _ := json.Marshal(map[string]string{"word": "ephemeral"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/vocabulary", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("returns 400 for missing word", func(t *testing.T) {
		store := &mockVocabStore{}
		controller := NewVocabularyController(store, nil, nil)

		router := newTestRouter(t)
		router.POST("/api/vocabulary", controller.AddWord)

		body, _ := json.Marshal(map[string]string{})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/vocabulary", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestVocabularyController_GetWord(t *testing.T) {
	t.Run("returns word", func(t *testing.T) {
		store := &mockVocabStore{word: &entities.Word{Word: "ephemeral"}}
		controller := NewVocabularyController(store, nil, nil)

		router := newTestRouter(t)
		router.GET("/api/vocabulary/:id", controller.GetWord)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/vocabulary/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 404 for missing word", func(t *testing.T) {
		store := &mockVocabStore{}
		controller := NewVocabularyController(store, nil, nil)

		router := newTestRouter(t)
		router.GET("/api/vocabulary/:id", controller.GetWord)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/vocabulary/999", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestVocabularyController_DeleteWord(t *testing.T) {
	store := &mockVocabStore{word: &entities.Word{Word: "test"}}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.DELETE("/api/vocabulary/:id", controller.DeleteWord)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/vocabulary/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVocabularyController_GetVocabularyStats(t *testing.T) {
	store := &mockVocabStore{statsTotal: 10, statsPend: 3, statsEnr: 5, statsFail: 2}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary/stats", controller.GetVocabularyStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(10), resp["total"])
	assert.Equal(t, float64(5), resp["enriched"])
}

func TestVocabularyController_SearchWords(t *testing.T) {
	store := &mockVocabStore{
		words: []entities.Word{{Word: "ephemeral"}},
	}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary/search", controller.SearchWords)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary/search?q=eph", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVocabularyController_GetWordsByHighlight(t *testing.T) {
	store := &mockVocabStore{words: []entities.Word{{Word: "test"}}}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/highlights/:id/vocabulary", controller.GetWordsByHighlight)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/highlights/1/vocabulary", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVocabularyController_UpdateWord(t *testing.T) {
	t.Run("updates word text", func(t *testing.T) {
		store := &mockVocabStore{word: &entities.Word{Word: "test"}}
		controller := NewVocabularyController(store, nil, nil)

		router := newTestRouter(t)
		router.PATCH("/api/vocabulary/:id", controller.UpdateWord)

		newWord := "updated"
		body, _ := json.Marshal(map[string]*string{"word": &newWord})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PATCH", "/api/vocabulary/1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 404 for missing word", func(t *testing.T) {
		store := &mockVocabStore{}
		controller := NewVocabularyController(store, nil, nil)

		router := newTestRouter(t)
		router.PATCH("/api/vocabulary/:id", controller.UpdateWord)

		body, _ := json.Marshal(map[string]string{"word": "new"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PATCH", "/api/vocabulary/999", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestVocabularyController_GetWord_HTMX(t *testing.T) {
	store := &mockVocabStore{word: &entities.Word{Word: "test"}}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary/:id", controller.GetWord)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary/1", nil)
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:word-detail")
}

func TestVocabularyController_DeleteWord_HTMX(t *testing.T) {
	store := &mockVocabStore{word: &entities.Word{Word: "test"}}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.DELETE("/api/vocabulary/:id", controller.DeleteWord)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/vocabulary/1", nil)
	req.Header.Set("HX-Request", "true")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVocabularyController_ListWords_WithStatus(t *testing.T) {
	store := &mockVocabStore{words: []entities.Word{{Word: "test"}}, total: 1}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary", controller.ListWords)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary?status=pending", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVocabularyController_ListWords_Pagination(t *testing.T) {
	store := &mockVocabStore{words: []entities.Word{}, total: 0}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/api/vocabulary", controller.ListWords)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/vocabulary?limit=10&offset=20", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(10), resp["limit"])
	assert.Equal(t, float64(20), resp["offset"])
}

func TestVocabularyController_VocabularyPage(t *testing.T) {
	store := &mockVocabStore{
		words:      []entities.Word{{Word: "test"}},
		total:      1,
		statsTotal: 1,
	}
	controller := NewVocabularyController(store, nil, nil)

	router := newTestRouter(t)
	router.GET("/vocabulary", controller.VocabularyPage)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/vocabulary", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TEMPLATE:vocabulary")
}
