package vocabulary

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/testutil"
)

func createTestWord(t *testing.T, db *gorm.DB, word string, status entities.WordStatus) *entities.Word {
	t.Helper()
	w := &entities.Word{
		Word:   word,
		Status: status,
	}
	require.NoError(t, db.Create(w).Error)
	return w
}

func TestRepository_AddWord(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	word := &entities.Word{
		Word:   "ephemeral",
		Status: entities.WordStatusPending,
	}

	err := repo.AddWord(word)

	require.NoError(t, err)
	assert.NotZero(t, word.ID)
}

func TestRepository_GetWordByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	created := createTestWord(t, db, "serendipity", entities.WordStatusPending)

	word, err := repo.GetWordByID(created.ID)

	require.NoError(t, err)
	assert.Equal(t, "serendipity", word.Word)
}

func TestRepository_GetWordByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetWordByID(999)

	assert.Error(t, err)
}

func TestRepository_UpdateWord(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	word := createTestWord(t, db, "ubiquitous", entities.WordStatusPending)
	word.Status = entities.WordStatusEnriched

	err := repo.UpdateWord(word)

	require.NoError(t, err)

	updated, _ := repo.GetWordByID(word.ID)
	assert.Equal(t, entities.WordStatusEnriched, updated.Status)
}

func TestRepository_DeleteWord(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	word := createTestWord(t, db, "to-delete", entities.WordStatusPending)

	err := repo.DeleteWord(word.ID)

	require.NoError(t, err)

	_, err = repo.GetWordByID(word.ID)
	assert.Error(t, err)
}

func TestRepository_GetAllWords_Pagination(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	for i := range 5 {
		createTestWord(t, db, "word"+string(rune('A'+i)), entities.WordStatusPending)
	}

	words, total, err := repo.GetAllWords(0, 2, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, words, 2)
}

func TestRepository_GetPendingWords(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	createTestWord(t, db, "pending1", entities.WordStatusPending)
	createTestWord(t, db, "pending2", entities.WordStatusPending)
	createTestWord(t, db, "enriched", entities.WordStatusEnriched)

	words, err := repo.GetPendingWords(10)

	require.NoError(t, err)
	assert.Len(t, words, 2)
}

func TestRepository_UpdateWordStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	word := createTestWord(t, db, "test", entities.WordStatusPending)

	err := repo.UpdateWordStatus(word.ID, entities.WordStatusFailed, "API error")

	require.NoError(t, err)

	updated, _ := repo.GetWordByID(word.ID)
	assert.Equal(t, entities.WordStatusFailed, updated.Status)
	assert.Equal(t, "API error", updated.EnrichmentError)
}

func TestRepository_SaveDefinitions(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	word := createTestWord(t, db, "test", entities.WordStatusPending)

	definitions := []entities.WordDefinition{
		{Definition: "first definition", PartOfSpeech: "noun"},
		{Definition: "second definition", PartOfSpeech: "verb"},
	}

	err := repo.SaveDefinitions(word.ID, definitions)

	require.NoError(t, err)

	updatedWord, _ := repo.GetWordByID(word.ID)
	assert.Len(t, updatedWord.Definitions, 2)
}

func TestRepository_SaveDefinitions_Replace(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	word := createTestWord(t, db, "test", entities.WordStatusPending)

	err := repo.SaveDefinitions(word.ID, []entities.WordDefinition{
		{Definition: "old definition"},
	})
	require.NoError(t, err)

	err = repo.SaveDefinitions(word.ID, []entities.WordDefinition{
		{Definition: "new definition 1"},
		{Definition: "new definition 2"},
	})
	require.NoError(t, err)

	updatedWord, _ := repo.GetWordByID(word.ID)
	assert.Len(t, updatedWord.Definitions, 2)
	assert.Equal(t, "new definition 1", updatedWord.Definitions[0].Definition)
}

func TestRepository_SearchWords(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	createTestWord(t, db, "ephemeral", entities.WordStatusPending)
	createTestWord(t, db, "serendipity", entities.WordStatusPending)

	words, err := repo.SearchWords("eph", 0, 10)

	require.NoError(t, err)
	assert.Len(t, words, 1)
	assert.Equal(t, "ephemeral", words[0].Word)
}

func TestRepository_GetVocabularyStats(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	createTestWord(t, db, "pending1", entities.WordStatusPending)
	createTestWord(t, db, "pending2", entities.WordStatusPending)
	createTestWord(t, db, "enriched", entities.WordStatusEnriched)
	createTestWord(t, db, "failed", entities.WordStatusFailed)

	total, pending, enriched, failed, err := repo.GetVocabularyStats(0)

	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	assert.Equal(t, int64(2), pending)
	assert.Equal(t, int64(1), enriched)
	assert.Equal(t, int64(1), failed)
}

func TestRepository_GetWordsByStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	createTestWord(t, db, "pending1", entities.WordStatusPending)
	createTestWord(t, db, "pending2", entities.WordStatusPending)
	createTestWord(t, db, "enriched", entities.WordStatusEnriched)

	words, total, err := repo.GetWordsByStatus(0, entities.WordStatusPending, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, words, 2)
}
