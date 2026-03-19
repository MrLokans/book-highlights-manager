package books_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/database/books"
	"github.com/mrlokans/assistant/internal/database/sources"
	"github.com/mrlokans/assistant/internal/database/tags"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// testDB holds the raw gorm.DB for the current test, set by setupListRepo.
var testDB *gorm.DB

func setupListRepo(t *testing.T) *books.Repository {
	t.Helper()
	dbPath := t.TempDir() + "/test_list.db"
	db, err := database.NewDatabase(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	testDB = db.DB
	sourcesRepo := sources.NewRepository(db.DB)
	return books.NewRepository(db.DB, sourcesRepo)
}

func getDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NotNil(t, testDB, "call setupListRepo before getDB")
	return testDB
}

func seedBooks(t *testing.T, repo *books.Repository, count int, userID uint) {
	t.Helper()
	for i := 0; i < count; i++ {
		b := &entities.Book{
			Title:  fmt.Sprintf("Book %03d", i+1),
			Author: fmt.Sprintf("Author %03d", i+1),
			UserID: userID,
		}
		require.NoError(t, repo.SaveBook(b))
		// Space out created_at so ordering is deterministic
		time.Sleep(time.Millisecond)
	}
}

func TestListBooks_DefaultSort(t *testing.T) {
	repo := setupListRepo(t)

	for i, title := range []string{"Alpha", "Bravo", "Charlie"} {
		b := &entities.Book{Title: title, Author: "Author", UserID: 1}
		require.NoError(t, repo.SaveBook(b))
		_ = i
		time.Sleep(2 * time.Millisecond)
	}

	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1})
	require.NoError(t, err)
	require.Len(t, result.Books, 3)

	// Default sort is created_at DESC, so newest first
	assert.Equal(t, "Charlie", result.Books[0].Title)
	assert.Equal(t, "Bravo", result.Books[1].Title)
	assert.Equal(t, "Alpha", result.Books[2].Title)
}

func TestListBooks_Pagination(t *testing.T) {
	repo := setupListRepo(t)
	seedBooks(t, repo, 25, 1)

	// Page 1 should return 20 items
	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1, Page: 1, PerPage: 20})
	require.NoError(t, err)
	assert.Len(t, result.Books, 20)
	assert.Equal(t, int64(25), result.TotalCount)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.TotalPages)

	// Page 2 should return 5 items
	result, err = repo.ListBooks(books.ListBooksOptions{UserID: 1, Page: 2, PerPage: 20})
	require.NoError(t, err)
	assert.Len(t, result.Books, 5)
	assert.Equal(t, int64(25), result.TotalCount)
	assert.Equal(t, 2, result.Page)
}

func TestListBooks_SortByTitle(t *testing.T) {
	repo := setupListRepo(t)

	for _, title := range []string{"Zebra", "Apple", "Mango"} {
		require.NoError(t, repo.SaveBook(&entities.Book{Title: title, Author: "Author", UserID: 1}))
	}

	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1, Sort: "title_asc"})
	require.NoError(t, err)
	require.Len(t, result.Books, 3)
	assert.Equal(t, "Apple", result.Books[0].Title)
	assert.Equal(t, "Mango", result.Books[1].Title)
	assert.Equal(t, "Zebra", result.Books[2].Title)
}

func TestListBooks_SortByHighlightCount(t *testing.T) {
	repo := setupListRepo(t)

	// Book with 0 highlights
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "No Highlights", Author: "A", UserID: 1}))
	// Book with 2 highlights
	require.NoError(t, repo.SaveBook(&entities.Book{
		Title: "Two Highlights", Author: "A", UserID: 1,
		Highlights: []entities.Highlight{{Text: "h1", UserID: 1}, {Text: "h2", UserID: 1}},
	}))
	// Book with 5 highlights
	require.NoError(t, repo.SaveBook(&entities.Book{
		Title: "Five Highlights", Author: "A", UserID: 1,
		Highlights: []entities.Highlight{
			{Text: "h1", UserID: 1}, {Text: "h2", UserID: 1}, {Text: "h3", UserID: 1},
			{Text: "h4", UserID: 1}, {Text: "h5", UserID: 1},
		},
	}))

	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1, Sort: "highlights_desc"})
	require.NoError(t, err)
	require.Len(t, result.Books, 3)
	assert.Equal(t, "Five Highlights", result.Books[0].Title)
	assert.Equal(t, int64(5), result.Books[0].HighlightCount)
	assert.Equal(t, "Two Highlights", result.Books[1].Title)
	assert.Equal(t, int64(2), result.Books[1].HighlightCount)
	assert.Equal(t, "No Highlights", result.Books[2].Title)
	assert.Equal(t, int64(0), result.Books[2].HighlightCount)
}

func TestListBooks_SearchFilter(t *testing.T) {
	repo := setupListRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "War and Peace", Author: "Tolstoy", UserID: 1}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "The Art of War", Author: "Sun Tzu", UserID: 1}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Crime and Punishment", Author: "Dostoevsky", UserID: 1}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Unrelated Book", Author: "Ed Warwick", UserID: 1}))

	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1, Query: "war"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.TotalCount) // 2 by title + 1 by author (Warwick)
}

func TestListBooks_TagFilter(t *testing.T) {
	repo := setupListRepo(t)

	b1 := &entities.Book{Title: "Tagged Book", Author: "A", UserID: 1}
	b2 := &entities.Book{Title: "Untagged Book", Author: "A", UserID: 1}
	require.NoError(t, repo.SaveBook(b1))
	require.NoError(t, repo.SaveBook(b2))

	// Create a tag and associate it with b1 via the tags repository
	db := getDB(t)
	tagsRepo := tags.NewRepository(db)
	tag, err := tagsRepo.CreateTag("fiction", 1)
	require.NoError(t, err)
	require.NoError(t, tagsRepo.AddTagToBook(b1.ID, tag.ID))

	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1, TagID: tag.ID})
	require.NoError(t, err)
	require.Len(t, result.Books, 1)
	assert.Equal(t, "Tagged Book", result.Books[0].Title)
}

func TestListBooks_InvalidPageClamped(t *testing.T) {
	repo := setupListRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "Only Book", Author: "A", UserID: 1}))

	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1, Page: 999, PerPage: 20})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 1, result.TotalPages)
	assert.Len(t, result.Books, 1)
}

func TestListBooks_CombinedFilters(t *testing.T) {
	repo := setupListRepo(t)

	// Create books with a searchable term
	for _, title := range []string{"Go Programming", "Go Patterns", "Go Concurrency", "Python Basics"} {
		require.NoError(t, repo.SaveBook(&entities.Book{Title: title, Author: "Author", UserID: 1}))
	}

	// Tag two of the Go books
	db := getDB(t)
	tagsRepo := tags.NewRepository(db)
	tag, err := tagsRepo.CreateTag("advanced", 1)
	require.NoError(t, err)

	// Find books to tag
	b1, err := repo.GetBookByTitleAndAuthorForUser("Go Patterns", "Author", 1)
	require.NoError(t, err)
	b2, err := repo.GetBookByTitleAndAuthorForUser("Go Concurrency", "Author", 1)
	require.NoError(t, err)
	require.NoError(t, tagsRepo.AddTagToBook(b1.ID, tag.ID))
	require.NoError(t, tagsRepo.AddTagToBook(b2.ID, tag.ID))

	// Query "Go" + tag "advanced" + title_asc + page 1 with perPage 1
	result, err := repo.ListBooks(books.ListBooksOptions{
		UserID:  1,
		Query:   "Go",
		TagID:   tag.ID,
		Sort:    "title_asc",
		Page:    1,
		PerPage: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.TotalCount)
	assert.Equal(t, 2, result.TotalPages)
	assert.Len(t, result.Books, 1)
	assert.Equal(t, "Go Concurrency", result.Books[0].Title)
}

func TestListBooks_UserScoping(t *testing.T) {
	repo := setupListRepo(t)

	require.NoError(t, repo.SaveBook(&entities.Book{Title: "User1 Book", Author: "A", UserID: 1}))
	require.NoError(t, repo.SaveBook(&entities.Book{Title: "User2 Book", Author: "A", UserID: 2}))

	result, err := repo.ListBooks(books.ListBooksOptions{UserID: 1})
	require.NoError(t, err)
	require.Len(t, result.Books, 1)
	assert.Equal(t, "User1 Book", result.Books[0].Title)
}
