package database

import (
	"os"
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTestDB creates a fresh test database
func setupTestDB(t *testing.T) (*Database, func()) {
	t.Helper()
	dbPath := "./test_" + t.Name() + ".db"
	db, err := NewDatabase(dbPath)
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}
	return db, cleanup
}

func TestDatabase(t *testing.T) {
	// Create a temporary database file
	dbPath := "./test.db"
	defer os.Remove(dbPath)

	// Initialize database
	db, err := NewDatabase(dbPath)
	require.NoError(t, err)
	defer db.Close()

	t.Run("SaveBook creates new book", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Test Book",
			Author: "Test Author",
			File:   "test.epub",
			Highlights: []entities.Highlight{
				{
					Time: time.Now().Format(time.RFC3339),
					Text: "This is a test highlight",
					Page: 42,
				},
			},
		}

		err := db.SaveBook(book)
		assert.NoError(t, err)
		assert.NotZero(t, book.ID)
		assert.NotZero(t, book.Highlights[0].ID)
		assert.Equal(t, book.ID, book.Highlights[0].BookID)
	})

	t.Run("GetBookByTitleAndAuthor retrieves saved book", func(t *testing.T) {
		retrievedBook, err := db.GetBookByTitleAndAuthor("Test Book", "Test Author")
		assert.NoError(t, err)
		assert.Equal(t, "Test Book", retrievedBook.Title)
		assert.Equal(t, "Test Author", retrievedBook.Author)
		assert.Len(t, retrievedBook.Highlights, 1)
		assert.Equal(t, "This is a test highlight", retrievedBook.Highlights[0].Text)
		assert.Equal(t, 42, retrievedBook.Highlights[0].Page) //nolint:staticcheck // Testing deprecated field for backward compatibility
	})

	t.Run("GetAllBooks returns all saved books", func(t *testing.T) {
		// Save another book
		book2 := &entities.Book{
			Title:  "Another Book",
			Author: "Another Author",
			File:   "another.epub",
			Highlights: []entities.Highlight{
				{
					Time: time.Now().Format(time.RFC3339),
					Text: "Another highlight",
					Page: 100,
				},
			},
		}

		err := db.SaveBook(book2)
		require.NoError(t, err)

		books, err := db.GetAllBooks()
		assert.NoError(t, err)
		assert.Len(t, books, 2)
	})

	t.Run("SaveBook updates existing book", func(t *testing.T) {
		// Get the existing book
		existingBook, err := db.GetBookByTitleAndAuthor("Test Book", "Test Author")
		require.NoError(t, err)

		// Add another highlight
		newHighlight := entities.Highlight{
			Time: time.Now().Format(time.RFC3339),
			Text: "Second highlight",
			Page: 84,
		}
		existingBook.Highlights = append(existingBook.Highlights, newHighlight)

		// Save the updated book
		err = db.SaveBook(existingBook)
		assert.NoError(t, err)

		// Retrieve and verify
		updatedBook, err := db.GetBookByTitleAndAuthor("Test Book", "Test Author")
		assert.NoError(t, err)
		assert.Len(t, updatedBook.Highlights, 2)
	})
}

// --- User Operations Tests ---

func TestUserOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("CreateUser creates user with token", func(t *testing.T) {
		user, err := db.CreateUser("testuser", "test@example.com")
		require.NoError(t, err)
		assert.NotZero(t, user.ID)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Len(t, user.Token, 64) // hex encoded 32 bytes
	})

	t.Run("GetUserByToken retrieves user", func(t *testing.T) {
		user, err := db.CreateUser("tokenuser", "token@example.com")
		require.NoError(t, err)

		retrieved, err := db.GetUserByToken(user.Token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, user.Username, retrieved.Username)
	})

	t.Run("GetUserByToken returns error for invalid token", func(t *testing.T) {
		_, err := db.GetUserByToken("nonexistent_token")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("GetUserByID retrieves user", func(t *testing.T) {
		user, err := db.CreateUser("iduser", "id@example.com")
		require.NoError(t, err)

		retrieved, err := db.GetUserByID(user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Username, retrieved.Username)
	})

	t.Run("GetUserByID returns error for nonexistent ID", func(t *testing.T) {
		_, err := db.GetUserByID(99999)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("GetUserByUsername retrieves user", func(t *testing.T) {
		user, err := db.CreateUser("uniqueuser", "unique@example.com")
		require.NoError(t, err)

		retrieved, err := db.GetUserByUsername("uniqueuser")
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
	})

	t.Run("GetUserByUsername returns error for nonexistent username", func(t *testing.T) {
		_, err := db.GetUserByUsername("nonexistent")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("CreateUser fails for duplicate username", func(t *testing.T) {
		_, err := db.CreateUser("dupuser", "dup1@example.com")
		require.NoError(t, err)

		_, err = db.CreateUser("dupuser", "dup2@example.com")
		assert.Error(t, err)
	})

	t.Run("CreateUser fails for duplicate email", func(t *testing.T) {
		_, err := db.CreateUser("emailuser1", "same@example.com")
		require.NoError(t, err)

		_, err = db.CreateUser("emailuser2", "same@example.com")
		assert.Error(t, err)
	})
}

// --- Tag Operations Tests ---

func TestTagOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a user for tags
	user, err := db.CreateUser("taguser", "taguser@example.com")
	require.NoError(t, err)

	t.Run("CreateTag creates new tag", func(t *testing.T) {
		tag, err := db.CreateTag("important", user.ID)
		require.NoError(t, err)
		assert.NotZero(t, tag.ID)
		assert.Equal(t, "important", tag.Name)
		assert.Equal(t, user.ID, tag.UserID)
	})

	t.Run("GetOrCreateTag returns existing tag", func(t *testing.T) {
		tag1, err := db.CreateTag("existing", user.ID)
		require.NoError(t, err)

		tag2, err := db.GetOrCreateTag("existing", user.ID)
		require.NoError(t, err)
		assert.Equal(t, tag1.ID, tag2.ID)
	})

	t.Run("GetOrCreateTag creates new tag if not exists", func(t *testing.T) {
		tag, err := db.GetOrCreateTag("newlycreated", user.ID)
		require.NoError(t, err)
		assert.NotZero(t, tag.ID)
		assert.Equal(t, "newlycreated", tag.Name)
	})

	t.Run("GetTagsForUser returns user tags", func(t *testing.T) {
		// Create another user with different tags
		user2, err := db.CreateUser("taguser2", "taguser2@example.com")
		require.NoError(t, err)

		_, err = db.CreateTag("user2tag", user2.ID)
		require.NoError(t, err)

		tags, err := db.GetTagsForUser(user.ID)
		require.NoError(t, err)
		// Should contain tags from user, not user2
		for _, tag := range tags {
			assert.Equal(t, user.ID, tag.UserID)
		}
	})

	t.Run("DeleteTag removes tag", func(t *testing.T) {
		tag, err := db.CreateTag("todelete", user.ID)
		require.NoError(t, err)

		err = db.DeleteTag(tag.ID)
		require.NoError(t, err)

		// Verify deletion
		tags, err := db.GetTagsForUser(user.ID)
		require.NoError(t, err)
		for _, existingTag := range tags {
			assert.NotEqual(t, tag.ID, existingTag.ID)
		}
	})
}

func TestTagHighlightAssociations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup user, book, and highlight
	user, err := db.CreateUser("assocuser", "assoc@example.com")
	require.NoError(t, err)

	book := &entities.Book{
		Title:  "Tag Test Book",
		Author: "Tag Author",
		UserID: user.ID,
		Highlights: []entities.Highlight{
			{
				Text:   "Highlight for tagging",
				UserID: user.ID,
			},
		},
	}
	err = db.SaveBook(book)
	require.NoError(t, err)

	highlightID := book.Highlights[0].ID

	t.Run("AddTagToHighlight associates tag with highlight", func(t *testing.T) {
		tag, err := db.CreateTag("testedtag", user.ID)
		require.NoError(t, err)

		err = db.AddTagToHighlight(highlightID, tag.ID)
		require.NoError(t, err)

		// Verify association
		highlight, err := db.GetHighlightByID(highlightID)
		require.NoError(t, err)
		assert.Len(t, highlight.Tags, 1)
		assert.Equal(t, tag.ID, highlight.Tags[0].ID)
	})

	t.Run("AddTagToHighlight fails for nonexistent highlight", func(t *testing.T) {
		tag, err := db.CreateTag("orphantag", user.ID)
		require.NoError(t, err)

		err = db.AddTagToHighlight(99999, tag.ID)
		assert.Error(t, err)
	})

	t.Run("AddTagToHighlight fails for nonexistent tag", func(t *testing.T) {
		err := db.AddTagToHighlight(highlightID, 99999)
		assert.Error(t, err)
	})

	t.Run("RemoveTagFromHighlight removes association", func(t *testing.T) {
		tag, err := db.CreateTag("removabletag", user.ID)
		require.NoError(t, err)

		err = db.AddTagToHighlight(highlightID, tag.ID)
		require.NoError(t, err)

		err = db.RemoveTagFromHighlight(highlightID, tag.ID)
		require.NoError(t, err)

		// Verify removal
		highlight, err := db.GetHighlightByID(highlightID)
		require.NoError(t, err)
		for _, existingTag := range highlight.Tags {
			assert.NotEqual(t, tag.ID, existingTag.ID)
		}
	})
}

// --- ImportSession Operations Tests ---

func TestImportSessionOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a user and get a source
	user, err := db.CreateUser("importuser", "import@example.com")
	require.NoError(t, err)

	source, err := db.GetSourceByName("readwise")
	require.NoError(t, err)

	t.Run("CreateImportSession creates new session", func(t *testing.T) {
		session, err := db.CreateImportSession(user.ID, source.ID)
		require.NoError(t, err)
		assert.NotZero(t, session.ID)
		assert.Equal(t, user.ID, session.UserID)
		assert.Equal(t, source.ID, session.SourceID)
		assert.Equal(t, entities.ImportStatusPending, session.Status)
	})

	t.Run("UpdateImportSession updates session", func(t *testing.T) {
		session, err := db.CreateImportSession(user.ID, source.ID)
		require.NoError(t, err)

		session.Status = entities.ImportStatusRunning
		session.BooksProcessed = 5
		session.HighlightsProcessed = 50

		err = db.UpdateImportSession(session)
		require.NoError(t, err)

		// Verify update
		retrieved, err := db.GetImportSession(session.ID)
		require.NoError(t, err)
		assert.Equal(t, entities.ImportStatusRunning, retrieved.Status)
		assert.Equal(t, 5, retrieved.BooksProcessed)
		assert.Equal(t, 50, retrieved.HighlightsProcessed)
	})

	t.Run("GetImportSession retrieves session with source", func(t *testing.T) {
		session, err := db.CreateImportSession(user.ID, source.ID)
		require.NoError(t, err)

		retrieved, err := db.GetImportSession(session.ID)
		require.NoError(t, err)
		assert.Equal(t, "readwise", retrieved.Source.Name)
	})

	t.Run("GetImportSession returns error for nonexistent ID", func(t *testing.T) {
		_, err := db.GetImportSession(99999)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("GetImportSessionsForUser returns user sessions", func(t *testing.T) {
		// Create another user with sessions
		user2, err := db.CreateUser("importuser2", "import2@example.com")
		require.NoError(t, err)

		_, err = db.CreateImportSession(user2.ID, source.ID)
		require.NoError(t, err)

		sessions, err := db.GetImportSessionsForUser(user.ID)
		require.NoError(t, err)
		for _, s := range sessions {
			assert.Equal(t, user.ID, s.UserID)
		}
	})

	t.Run("GetImportSessionsForUser orders by started_at desc", func(t *testing.T) {
		// Create multiple sessions
		for i := 0; i < 3; i++ {
			_, err := db.CreateImportSession(user.ID, source.ID)
			require.NoError(t, err)
		}

		sessions, err := db.GetImportSessionsForUser(user.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(sessions), 3)

		// Check ordering (most recent first)
		for i := 1; i < len(sessions); i++ {
			assert.True(t, sessions[i-1].StartedAt.After(sessions[i].StartedAt) ||
				sessions[i-1].StartedAt.Equal(sessions[i].StartedAt))
		}
	})
}

// --- Statistics Tests ---

func TestStatistics(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("GetStats returns global counts", func(t *testing.T) {
		// Create some books with highlights
		book1 := &entities.Book{
			Title:  "Stats Book 1",
			Author: "Stats Author",
			Highlights: []entities.Highlight{
				{Text: "Highlight 1"},
				{Text: "Highlight 2"},
			},
		}
		book2 := &entities.Book{
			Title:  "Stats Book 2",
			Author: "Stats Author",
			Highlights: []entities.Highlight{
				{Text: "Highlight 3"},
			},
		}
		err := db.SaveBook(book1)
		require.NoError(t, err)
		err = db.SaveBook(book2)
		require.NoError(t, err)

		totalBooks, totalHighlights, err := db.GetStats()
		require.NoError(t, err)
		assert.Equal(t, int64(2), totalBooks)
		assert.Equal(t, int64(3), totalHighlights)
	})

	t.Run("GetStatsForUser returns user-specific counts", func(t *testing.T) {
		user1, err := db.CreateUser("statsuser1", "stats1@example.com")
		require.NoError(t, err)
		user2, err := db.CreateUser("statsuser2", "stats2@example.com")
		require.NoError(t, err)

		// Create books for user1
		book1 := &entities.Book{
			Title:  "User1 Book",
			Author: "Author",
			UserID: user1.ID,
			Highlights: []entities.Highlight{
				{Text: "User1 Highlight 1", UserID: user1.ID},
				{Text: "User1 Highlight 2", UserID: user1.ID},
			},
		}
		err = db.SaveBook(book1)
		require.NoError(t, err)

		// Create books for user2
		book2 := &entities.Book{
			Title:  "User2 Book",
			Author: "Author",
			UserID: user2.ID,
			Highlights: []entities.Highlight{
				{Text: "User2 Highlight", UserID: user2.ID},
			},
		}
		err = db.SaveBook(book2)
		require.NoError(t, err)

		// Check user1 stats
		books1, highlights1, err := db.GetStatsForUser(user1.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), books1)
		assert.Equal(t, int64(2), highlights1)

		// Check user2 stats
		books2, highlights2, err := db.GetStatsForUser(user2.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), books2)
		assert.Equal(t, int64(1), highlights2)
	})
}

// --- Setting Operations Tests ---

func TestSettingOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("SetSetting creates new setting", func(t *testing.T) {
		err := db.SetSetting("test_key", "test_value")
		require.NoError(t, err)

		setting, err := db.GetSetting("test_key")
		require.NoError(t, err)
		assert.Equal(t, "test_key", setting.Key)
		assert.Equal(t, "test_value", setting.Value)
	})

	t.Run("SetSetting updates existing setting", func(t *testing.T) {
		err := db.SetSetting("update_key", "initial_value")
		require.NoError(t, err)

		err = db.SetSetting("update_key", "updated_value")
		require.NoError(t, err)

		setting, err := db.GetSetting("update_key")
		require.NoError(t, err)
		assert.Equal(t, "updated_value", setting.Value)
	})

	t.Run("GetSetting returns error for nonexistent key", func(t *testing.T) {
		_, err := db.GetSetting("nonexistent_key")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("DeleteSetting removes setting", func(t *testing.T) {
		err := db.SetSetting("delete_key", "to_delete")
		require.NoError(t, err)

		err = db.DeleteSetting("delete_key")
		require.NoError(t, err)

		_, err = db.GetSetting("delete_key")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("DeleteSetting does not error for nonexistent key", func(t *testing.T) {
		err := db.DeleteSetting("never_existed")
		assert.NoError(t, err)
	})
}

// --- Source Operations Tests ---

func TestSourceOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("GetSourceByName returns seeded source", func(t *testing.T) {
		source, err := db.GetSourceByName("kindle")
		require.NoError(t, err)
		assert.Equal(t, "kindle", source.Name)
		assert.Equal(t, "Amazon Kindle", source.DisplayName)
	})

	t.Run("GetSourceByName returns error for unknown source", func(t *testing.T) {
		_, err := db.GetSourceByName("unknown_source")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("GetAllSources returns all seeded sources", func(t *testing.T) {
		sources, err := db.GetAllSources()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(sources), len(defaultSources))

		// Check that expected sources exist
		expectedNames := []string{"readwise", "kindle", "apple_books", "moonreader"}
		sourceNames := make(map[string]bool)
		for _, s := range sources {
			sourceNames[s.Name] = true
		}
		for _, name := range expectedNames {
			assert.True(t, sourceNames[name], "Expected source %s not found", name)
		}
	})
}

// --- Book Operations Extended Tests ---

func TestBookOperationsExtended(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	user, err := db.CreateUser("bookuser", "book@example.com")
	require.NoError(t, err)

	t.Run("SaveBookForUser sets UserID", func(t *testing.T) {
		book := &entities.Book{
			Title:  "User Book",
			Author: "User Author",
		}
		err := db.SaveBookForUser(book, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, book.UserID)
	})

	t.Run("GetAllBooksForUser returns only user books", func(t *testing.T) {
		user2, err := db.CreateUser("bookuser2", "book2@example.com")
		require.NoError(t, err)

		book2 := &entities.Book{
			Title:  "User2 Book",
			Author: "User2 Author",
			UserID: user2.ID,
		}
		err = db.SaveBook(book2)
		require.NoError(t, err)

		books, err := db.GetAllBooksForUser(user.ID)
		require.NoError(t, err)
		for _, b := range books {
			assert.Equal(t, user.ID, b.UserID)
		}
	})

	t.Run("GetBookByTitleAndAuthorForUser respects user", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Unique Per User",
			Author: "Author",
			UserID: user.ID,
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		retrieved, err := db.GetBookByTitleAndAuthorForUser("Unique Per User", "Author", user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.UserID)

		// Different user should not find it
		_, err = db.GetBookByTitleAndAuthorForUser("Unique Per User", "Author", 99999)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("GetBookByID retrieves book with highlights ordered by location", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Ordered Highlights Book",
			Author: "Order Author",
			UserID: user.ID,
			Highlights: []entities.Highlight{
				{Text: "Third", LocationValue: 300, UserID: user.ID},
				{Text: "First", LocationValue: 100, UserID: user.ID},
				{Text: "Second", LocationValue: 200, UserID: user.ID},
			},
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		retrieved, err := db.GetBookByID(book.ID)
		require.NoError(t, err)
		require.Len(t, retrieved.Highlights, 3)
		assert.Equal(t, "First", retrieved.Highlights[0].Text)
		assert.Equal(t, "Second", retrieved.Highlights[1].Text)
		assert.Equal(t, "Third", retrieved.Highlights[2].Text)
	})

	t.Run("SearchBooks finds by title", func(t *testing.T) {
		book := &entities.Book{
			Title:  "SearchableTitle",
			Author: "SearchAuthor",
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		books, err := db.SearchBooks("SearchableTitle")
		require.NoError(t, err)
		assert.NotEmpty(t, books)
		found := false
		for _, b := range books {
			if b.Title == "SearchableTitle" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("SearchBooks finds by author", func(t *testing.T) {
		books, err := db.SearchBooks("SearchAuthor")
		require.NoError(t, err)
		assert.NotEmpty(t, books)
	})

	t.Run("SearchBooks is case insensitive", func(t *testing.T) {
		books, err := db.SearchBooks("searchabletitle")
		require.NoError(t, err)
		assert.NotEmpty(t, books)
	})

	t.Run("DeleteBook soft deletes book", func(t *testing.T) {
		book := &entities.Book{
			Title:  "ToDelete",
			Author: "DeleteAuthor",
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		err = db.DeleteBook(book.ID)
		require.NoError(t, err)

		_, err = db.GetBookByID(book.ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("FindBookByISBN finds book", func(t *testing.T) {
		book := &entities.Book{
			Title:  "ISBN Book",
			Author: "ISBN Author",
			ISBN:   "978-0-13-468599-1",
			UserID: user.ID,
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		found, err := db.FindBookByISBN("978-0-13-468599-1", user.ID)
		require.NoError(t, err)
		assert.Equal(t, book.ID, found.ID)
	})

	t.Run("FindBookByFileHash finds book", func(t *testing.T) {
		book := &entities.Book{
			Title:    "Hash Book",
			Author:   "Hash Author",
			FileHash: "abc123hash",
			UserID:   user.ID,
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		found, err := db.FindBookByFileHash("abc123hash", user.ID)
		require.NoError(t, err)
		assert.Equal(t, book.ID, found.ID)
	})
}

// --- Highlight Operations Tests ---

func TestHighlightOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	user, err := db.CreateUser("highlightuser", "highlight@example.com")
	require.NoError(t, err)

	book := &entities.Book{
		Title:  "Highlight Test Book",
		Author: "Highlight Author",
		UserID: user.ID,
		Highlights: []entities.Highlight{
			{Text: "First Highlight", LocationValue: 100, UserID: user.ID},
			{Text: "Second Highlight", LocationValue: 200, UserID: user.ID},
		},
	}
	err = db.SaveBook(book)
	require.NoError(t, err)

	t.Run("GetHighlightByID retrieves highlight with tags", func(t *testing.T) {
		highlight, err := db.GetHighlightByID(book.Highlights[0].ID)
		require.NoError(t, err)
		assert.Equal(t, "First Highlight", highlight.Text)
	})

	t.Run("GetHighlightsForBook returns ordered highlights", func(t *testing.T) {
		highlights, err := db.GetHighlightsForBook(book.ID)
		require.NoError(t, err)
		require.Len(t, highlights, 2)
		assert.Equal(t, "First Highlight", highlights[0].Text)
		assert.Equal(t, "Second Highlight", highlights[1].Text)
	})

	t.Run("GetHighlightsForUser with pagination", func(t *testing.T) {
		highlights, err := db.GetHighlightsForUser(user.ID, 1, 0)
		require.NoError(t, err)
		assert.Len(t, highlights, 1)

		highlights, err = db.GetHighlightsForUser(user.ID, 10, 1)
		require.NoError(t, err)
		assert.Len(t, highlights, 1) // Only 1 left after offset
	})

	t.Run("UpdateHighlight modifies highlight", func(t *testing.T) {
		highlight, err := db.GetHighlightByID(book.Highlights[0].ID)
		require.NoError(t, err)

		highlight.Note = "Added a note"
		err = db.UpdateHighlight(highlight)
		require.NoError(t, err)

		updated, err := db.GetHighlightByID(book.Highlights[0].ID)
		require.NoError(t, err)
		assert.Equal(t, "Added a note", updated.Note)
	})

	t.Run("DeleteHighlight soft deletes highlight", func(t *testing.T) {
		err := db.DeleteHighlight(book.Highlights[1].ID)
		require.NoError(t, err)

		_, err = db.GetHighlightByID(book.Highlights[1].ID)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

// --- Book Save with Source Tests ---

func TestBookSaveWithSource(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("SaveBook looks up source by name", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Source Test Book",
			Author: "Source Author",
			Source: entities.Source{Name: "kindle"},
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		retrieved, err := db.GetBookByID(book.ID)
		require.NoError(t, err)
		assert.NotZero(t, retrieved.SourceID)
		assert.Equal(t, "kindle", retrieved.Source.Name)
	})

	t.Run("SaveBook preserves source info after save", func(t *testing.T) {
		book := &entities.Book{
			Title:  "Preserved Source Book",
			Author: "Preserved Author",
			Source: entities.Source{Name: "apple_books"},
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		// Source should be preserved on the original object
		assert.Equal(t, "apple_books", book.Source.Name)
	})

	t.Run("SaveBook deduplicates highlights", func(t *testing.T) {
		now := time.Now()
		book := &entities.Book{
			Title:  "Dedup Book",
			Author: "Dedup Author",
			Highlights: []entities.Highlight{
				{Text: "Same text", LocationValue: 100, HighlightedAt: now},
			},
		}
		err := db.SaveBook(book)
		require.NoError(t, err)

		// Save same book again with same highlight
		book2 := &entities.Book{
			Title:  "Dedup Book",
			Author: "Dedup Author",
			Highlights: []entities.Highlight{
				{Text: "Same text", LocationValue: 100, HighlightedAt: now},
				{Text: "New text", LocationValue: 200, HighlightedAt: now},
			},
		}
		err = db.SaveBook(book2)
		require.NoError(t, err)

		// Should have 2 highlights, not 3
		retrieved, err := db.GetBookByTitleAndAuthor("Dedup Book", "Dedup Author")
		require.NoError(t, err)
		assert.Len(t, retrieved.Highlights, 2)
	})
}

// --- Database Initialization Tests ---

func TestDatabaseInitialization(t *testing.T) {
	t.Run("NewDatabase creates database file", func(t *testing.T) {
		dbPath := "./init_test.db"
		defer os.Remove(dbPath)

		db, err := NewDatabase(dbPath)
		require.NoError(t, err)
		defer db.Close()

		// File should exist
		_, err = os.Stat(dbPath)
		assert.NoError(t, err)
	})

	t.Run("NewDatabase seeds sources on creation", func(t *testing.T) {
		dbPath := "./seed_test.db"
		defer os.Remove(dbPath)

		db, err := NewDatabase(dbPath)
		require.NoError(t, err)
		defer db.Close()

		sources, err := db.GetAllSources()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(sources), len(defaultSources))
	})

	t.Run("NewDatabase is idempotent for sources", func(t *testing.T) {
		dbPath := "./idempotent_test.db"
		defer os.Remove(dbPath)

		// Create and close
		db1, err := NewDatabase(dbPath)
		require.NoError(t, err)
		sources1, _ := db1.GetAllSources()
		db1.Close()

		// Reopen - should not duplicate sources
		db2, err := NewDatabase(dbPath)
		require.NoError(t, err)
		defer db2.Close()

		sources2, err := db2.GetAllSources()
		require.NoError(t, err)
		assert.Equal(t, len(sources1), len(sources2))
	})

	t.Run("Close closes database connection", func(t *testing.T) {
		dbPath := "./close_test.db"
		defer os.Remove(dbPath)

		db, err := NewDatabase(dbPath)
		require.NoError(t, err)

		err = db.Close()
		assert.NoError(t, err)
	})
}
