package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVocabularyTestDB(t *testing.T) (*Database, func()) {
	tmpDir, err := os.MkdirTemp("", "vocab_test")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := NewDatabase(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestAddWord(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	word := &entities.Word{
		Word:   "ephemeral",
		Status: entities.WordStatusPending,
	}

	err := db.AddWord(word)
	require.NoError(t, err)
	assert.NotZero(t, word.ID)

	// Verify retrieval
	retrieved, err := db.GetWordByID(word.ID)
	require.NoError(t, err)
	assert.Equal(t, "ephemeral", retrieved.Word)
	assert.Equal(t, entities.WordStatusPending, retrieved.Status)
}

func TestGetAllWords(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	// Add multiple words
	words := []string{"apple", "banana", "cherry"}
	for _, w := range words {
		err := db.AddWord(&entities.Word{Word: w, Status: entities.WordStatusPending})
		require.NoError(t, err)
	}

	// Get all
	result, total, err := db.GetAllWords(0, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, result, 3)

	// Test pagination
	result, _, err = db.GetAllWords(0, 2, 0)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestUpdateWordStatus(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	word := &entities.Word{
		Word:   "test",
		Status: entities.WordStatusPending,
	}
	require.NoError(t, db.AddWord(word))

	// Update to enriched
	err := db.UpdateWordStatus(word.ID, entities.WordStatusEnriched, "")
	require.NoError(t, err)

	retrieved, err := db.GetWordByID(word.ID)
	require.NoError(t, err)
	assert.Equal(t, entities.WordStatusEnriched, retrieved.Status)
	assert.Empty(t, retrieved.EnrichmentError)

	// Update to failed with error message
	err = db.UpdateWordStatus(word.ID, entities.WordStatusFailed, "API error")
	require.NoError(t, err)

	retrieved, err = db.GetWordByID(word.ID)
	require.NoError(t, err)
	assert.Equal(t, entities.WordStatusFailed, retrieved.Status)
	assert.Equal(t, "API error", retrieved.EnrichmentError)
}

func TestDeleteWord(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	word := &entities.Word{Word: "delete_me", Status: entities.WordStatusPending}
	require.NoError(t, db.AddWord(word))

	// Add definitions
	defs := []entities.WordDefinition{
		{WordID: word.ID, PartOfSpeech: "noun", Definition: "A test definition"},
	}
	require.NoError(t, db.SaveDefinitions(word.ID, defs))

	// Delete
	err := db.DeleteWord(word.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = db.GetWordByID(word.ID)
	assert.Error(t, err)
}

func TestGetPendingWords(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	// Add words with different statuses
	_ = db.AddWord(&entities.Word{Word: "pending1", Status: entities.WordStatusPending})
	_ = db.AddWord(&entities.Word{Word: "pending2", Status: entities.WordStatusPending})
	_ = db.AddWord(&entities.Word{Word: "enriched", Status: entities.WordStatusEnriched})
	_ = db.AddWord(&entities.Word{Word: "failed", Status: entities.WordStatusFailed})

	pending, err := db.GetPendingWords(0)
	require.NoError(t, err)
	assert.Len(t, pending, 2)

	for _, w := range pending {
		assert.Equal(t, entities.WordStatusPending, w.Status)
	}
}

func TestSaveDefinitions(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	word := &entities.Word{Word: "test", Status: entities.WordStatusPending}
	require.NoError(t, db.AddWord(word))

	// Save definitions
	defs := []entities.WordDefinition{
		{PartOfSpeech: "noun", Definition: "First definition"},
		{PartOfSpeech: "verb", Definition: "Second definition"},
	}
	err := db.SaveDefinitions(word.ID, defs)
	require.NoError(t, err)

	// Verify
	retrieved, err := db.GetWordByID(word.ID)
	require.NoError(t, err)
	assert.Len(t, retrieved.Definitions, 2)

	// Replace definitions
	newDefs := []entities.WordDefinition{
		{PartOfSpeech: "adjective", Definition: "New definition"},
	}
	err = db.SaveDefinitions(word.ID, newDefs)
	require.NoError(t, err)

	retrieved, err = db.GetWordByID(word.ID)
	require.NoError(t, err)
	assert.Len(t, retrieved.Definitions, 1)
	assert.Equal(t, "adjective", retrieved.Definitions[0].PartOfSpeech)
}

func TestFindWordBySource(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	word := &entities.Word{
		Word:                "unique",
		Status:              entities.WordStatusPending,
		SourceBookTitle:     "Test Book",
		SourceBookAuthor:    "Test Author",
		SourceHighlightText: "This is a test highlight.",
	}
	require.NoError(t, db.AddWord(word))

	// Find existing
	found, err := db.FindWordBySource("unique", "Test Book", "Test Author", "This is a test highlight.", 0)
	require.NoError(t, err)
	assert.Equal(t, word.ID, found.ID)

	// Not found
	_, err = db.FindWordBySource("unique", "Different Book", "Test Author", "This is a test highlight.", 0)
	assert.Error(t, err)
}

func TestSearchWords(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	_ = db.AddWord(&entities.Word{Word: "ephemeral", Status: entities.WordStatusPending})
	_ = db.AddWord(&entities.Word{Word: "ephemeron", Status: entities.WordStatusPending})
	_ = db.AddWord(&entities.Word{Word: "different", Status: entities.WordStatusPending})

	// Search
	results, err := db.SearchWords("ephem", 0, 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Case insensitive
	results, err = db.SearchWords("EPHEM", 0, 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGetVocabularyStats(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	_ = db.AddWord(&entities.Word{Word: "pending1", Status: entities.WordStatusPending})
	_ = db.AddWord(&entities.Word{Word: "pending2", Status: entities.WordStatusPending})
	_ = db.AddWord(&entities.Word{Word: "enriched", Status: entities.WordStatusEnriched})
	_ = db.AddWord(&entities.Word{Word: "failed", Status: entities.WordStatusFailed})

	total, pending, enriched, failed, err := db.GetVocabularyStats(0)
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	assert.Equal(t, int64(2), pending)
	assert.Equal(t, int64(1), enriched)
	assert.Equal(t, int64(1), failed)
}

func TestGetWordsByHighlight(t *testing.T) {
	db, cleanup := setupVocabularyTestDB(t)
	defer cleanup()

	// Create book and highlight first
	book := &entities.Book{
		Title:  "Test Book",
		Author: "Test Author",
		Highlights: []entities.Highlight{
			{Text: "Test highlight"},
		},
	}
	require.NoError(t, db.SaveBook(book))

	highlightID := book.Highlights[0].ID

	// Add words linked to highlight
	word1 := &entities.Word{Word: "word1", HighlightID: &highlightID, Status: entities.WordStatusPending}
	word2 := &entities.Word{Word: "word2", HighlightID: &highlightID, Status: entities.WordStatusPending}
	word3 := &entities.Word{Word: "word3", Status: entities.WordStatusPending} // Not linked
	require.NoError(t, db.AddWord(word1))
	require.NoError(t, db.AddWord(word2))
	require.NoError(t, db.AddWord(word3))

	words, err := db.GetWordsByHighlight(highlightID)
	require.NoError(t, err)
	assert.Len(t, words, 2)
}
