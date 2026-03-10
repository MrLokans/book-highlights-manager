package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/testutil"
)

func TestRepository_CreateTag(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag, err := repo.CreateTag("fiction", 1)

	require.NoError(t, err)
	assert.NotZero(t, tag.ID)
	assert.Equal(t, "fiction", tag.Name)
	assert.Equal(t, uint(1), tag.UserID)
}

func TestRepository_CreateTag_DuplicateName(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.CreateTag("fiction", 1)
	require.NoError(t, err)

	// Unique constraint on (user_id, name) should reject duplicate
	_, err = repo.CreateTag("fiction", 1)
	assert.Error(t, err)
}

func TestRepository_CreateTag_EmptyName(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag, err := repo.CreateTag("", 1)
	// Empty names are allowed at the DB level — verify behavior
	require.NoError(t, err)
	assert.Equal(t, "", tag.Name)
}

func TestRepository_GetOrCreateTag_New(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag, err := repo.GetOrCreateTag("science", 1)

	require.NoError(t, err)
	assert.NotZero(t, tag.ID)
	assert.Equal(t, "science", tag.Name)
}

func TestRepository_GetOrCreateTag_Existing(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag1, err := repo.CreateTag("history", 1)
	require.NoError(t, err)

	tag2, err := repo.GetOrCreateTag("history", 1)
	require.NoError(t, err)
	assert.Equal(t, tag1.ID, tag2.ID)
}

func TestRepository_GetOrCreateTag_CaseInsensitive(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag1, err := repo.CreateTag("Fiction", 1)
	require.NoError(t, err)

	tag2, err := repo.GetOrCreateTag("fiction", 1)
	require.NoError(t, err)
	assert.Equal(t, tag1.ID, tag2.ID)
}

func TestRepository_GetOrCreateTag_DifferentUsers(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag1, err := repo.GetOrCreateTag("fiction", 1)
	require.NoError(t, err)

	// Same name but different user — should create a new tag
	tag2, err := repo.GetOrCreateTag("fiction", 2)
	require.NoError(t, err)
	assert.NotEqual(t, tag1.ID, tag2.ID)
}

func TestRepository_GetTagByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	created, err := repo.CreateTag("test-tag", 1)
	require.NoError(t, err)

	found, err := repo.GetTagByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "test-tag", found.Name)
}

func TestRepository_GetTagByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetTagByID(99999)
	assert.Error(t, err)
}

func TestRepository_GetTagsForUser(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.CreateTag("user1-tag", 1)
	require.NoError(t, err)
	_, err = repo.CreateTag("user2-tag", 2)
	require.NoError(t, err)

	tags, err := repo.GetTagsForUser(1)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.Equal(t, "user1-tag", tags[0].Name)
}

func TestRepository_GetTagsForUser_Empty(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tags, err := repo.GetTagsForUser(1)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestRepository_SearchTags(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.CreateTag("science-fiction", 1)
	require.NoError(t, err)
	_, err = repo.CreateTag("history", 1)
	require.NoError(t, err)

	tags, err := repo.SearchTags("fic", 1)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.Equal(t, "science-fiction", tags[0].Name)
}

func TestRepository_SearchTags_CaseInsensitive(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.CreateTag("Science-Fiction", 1)
	require.NoError(t, err)

	tags, err := repo.SearchTags("science", 1)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
}

func TestRepository_SearchTags_NoResults(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.CreateTag("history", 1)
	require.NoError(t, err)

	tags, err := repo.SearchTags("xyz", 1)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestRepository_SearchTags_DoesNotCrossUsers(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.CreateTag("fiction", 1)
	require.NoError(t, err)

	tags, err := repo.SearchTags("fic", 2)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestRepository_DeleteTag(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag, err := repo.CreateTag("to-delete", 1)
	require.NoError(t, err)

	err = repo.DeleteTag(tag.ID)
	require.NoError(t, err)

	_, err = repo.GetTagByID(tag.ID)
	assert.Error(t, err)
}

func TestRepository_DeleteTag_NonExistent(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	// Deleting a non-existent tag should not error (GORM soft behavior)
	err := repo.DeleteTag(99999)
	assert.NoError(t, err)
}

func TestRepository_IsTagOrphan(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag, err := repo.CreateTag("orphan-tag", 1)
	require.NoError(t, err)

	isOrphan, err := repo.IsTagOrphan(tag.ID)
	require.NoError(t, err)
	assert.True(t, isOrphan)
}

func TestRepository_IsTagOrphan_WithBookAssociation(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	tag := testutil.SeedTag(t, db, "linked-tag", user.ID)

	err := repo.AddTagToBook(book.ID, tag.ID)
	require.NoError(t, err)

	isOrphan, err := repo.IsTagOrphan(tag.ID)
	require.NoError(t, err)
	assert.False(t, isOrphan)
}

func TestRepository_IsTagOrphan_WithHighlightAssociation(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	highlight := testutil.SeedHighlight(t, db, book.ID, "some highlight text")
	tag := testutil.SeedTag(t, db, "highlight-tag", user.ID)

	err := repo.AddTagToHighlight(highlight.ID, tag.ID)
	require.NoError(t, err)

	isOrphan, err := repo.IsTagOrphan(tag.ID)
	require.NoError(t, err)
	assert.False(t, isOrphan)
}

func TestRepository_DeleteTagIfOrphan_Orphan(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag, err := repo.CreateTag("orphan", 1)
	require.NoError(t, err)

	err = repo.DeleteTagIfOrphan(tag.ID)
	require.NoError(t, err)

	_, err = repo.GetTagByID(tag.ID)
	assert.Error(t, err)
}

func TestRepository_DeleteTagIfOrphan_NotOrphan(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	tag := testutil.SeedTag(t, db, "linked", user.ID)

	err := repo.AddTagToBook(book.ID, tag.ID)
	require.NoError(t, err)

	err = repo.DeleteTagIfOrphan(tag.ID)
	require.NoError(t, err)

	// Tag should still exist
	found, err := repo.GetTagByID(tag.ID)
	require.NoError(t, err)
	assert.Equal(t, tag.ID, found.ID)
}

func TestRepository_DeleteOrphanTags(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.CreateTag("orphan1", 1)
	require.NoError(t, err)
	_, err = repo.CreateTag("orphan2", 1)
	require.NoError(t, err)

	deleted, err := repo.DeleteOrphanTags()
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	tags, err := repo.GetTagsForUser(1)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestRepository_DeleteOrphanTags_PreservesLinkedTags(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)

	orphanTag := testutil.SeedTag(t, db, "orphan", user.ID)
	linkedTag := testutil.SeedTag(t, db, "linked", user.ID)

	err := repo.AddTagToBook(book.ID, linkedTag.ID)
	require.NoError(t, err)

	deleted, err := repo.DeleteOrphanTags()
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Orphan should be gone
	_, err = repo.GetTagByID(orphanTag.ID)
	assert.Error(t, err)

	// Linked should remain
	_, err = repo.GetTagByID(linkedTag.ID)
	assert.NoError(t, err)
}

func TestRepository_DeleteOrphanTags_NoneToDelete(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	deleted, err := repo.DeleteOrphanTags()
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

func TestRepository_AddTagToBook(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	tag := testutil.SeedTag(t, db, "my-tag", user.ID)

	err := repo.AddTagToBook(book.ID, tag.ID)
	require.NoError(t, err)

	// Verify via GetBookByID
	found, err := repo.GetBookByID(book.ID)
	require.NoError(t, err)
	require.Len(t, found.Tags, 1)
	assert.Equal(t, tag.ID, found.Tags[0].ID)
}

func TestRepository_AddTagToBook_NonExistentBook(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag := testutil.SeedTag(t, db, "tag", 1)

	err := repo.AddTagToBook(99999, tag.ID)
	assert.Error(t, err)
}

func TestRepository_AddTagToBook_NonExistentTag(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)

	err := repo.AddTagToBook(book.ID, 99999)
	assert.Error(t, err)
}

func TestRepository_RemoveTagFromBook(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	tag := testutil.SeedTag(t, db, "removable", user.ID)

	err := repo.AddTagToBook(book.ID, tag.ID)
	require.NoError(t, err)

	err = repo.RemoveTagFromBook(book.ID, tag.ID)
	require.NoError(t, err)

	// Tag should be deleted (was orphaned after removal)
	_, err = repo.GetTagByID(tag.ID)
	assert.Error(t, err)
}

func TestRepository_RemoveTagFromBook_TagStillLinkedElsewhere(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book1 := testutil.SeedBook(t, db, "Book 1", "Author", source.ID, user.ID)
	book2 := testutil.SeedBook(t, db, "Book 2", "Author", source.ID, user.ID)
	tag := testutil.SeedTag(t, db, "shared-tag", user.ID)

	require.NoError(t, repo.AddTagToBook(book1.ID, tag.ID))
	require.NoError(t, repo.AddTagToBook(book2.ID, tag.ID))

	err := repo.RemoveTagFromBook(book1.ID, tag.ID)
	require.NoError(t, err)

	// Tag should still exist (linked to book2)
	found, err := repo.GetTagByID(tag.ID)
	require.NoError(t, err)
	assert.Equal(t, tag.ID, found.ID)
}

func TestRepository_RemoveTagFromBook_NonExistentBook(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag := testutil.SeedTag(t, db, "tag", 1)

	err := repo.RemoveTagFromBook(99999, tag.ID)
	assert.Error(t, err)
}

func TestRepository_AddTagToHighlight(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	highlight := testutil.SeedHighlight(t, db, book.ID, "important text")
	tag := testutil.SeedTag(t, db, "my-tag", user.ID)

	err := repo.AddTagToHighlight(highlight.ID, tag.ID)
	require.NoError(t, err)

	found, err := repo.GetHighlightByID(highlight.ID)
	require.NoError(t, err)
	require.Len(t, found.Tags, 1)
	assert.Equal(t, tag.ID, found.Tags[0].ID)
}

func TestRepository_AddTagToHighlight_NonExistentHighlight(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag := testutil.SeedTag(t, db, "tag", 1)

	err := repo.AddTagToHighlight(99999, tag.ID)
	assert.Error(t, err)
}

func TestRepository_AddTagToHighlight_NonExistentTag(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	highlight := testutil.SeedHighlight(t, db, book.ID, "text")

	err := repo.AddTagToHighlight(highlight.ID, 99999)
	assert.Error(t, err)
}

func TestRepository_RemoveTagFromHighlight(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Test Book", "Author", source.ID, user.ID)
	highlight := testutil.SeedHighlight(t, db, book.ID, "text")
	tag := testutil.SeedTag(t, db, "removable", user.ID)

	require.NoError(t, repo.AddTagToHighlight(highlight.ID, tag.ID))

	err := repo.RemoveTagFromHighlight(highlight.ID, tag.ID)
	require.NoError(t, err)

	// Tag should be deleted (orphaned after removal)
	_, err = repo.GetTagByID(tag.ID)
	assert.Error(t, err)
}

func TestRepository_RemoveTagFromHighlight_NonExistentHighlight(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	tag := testutil.SeedTag(t, db, "tag", 1)

	err := repo.RemoveTagFromHighlight(99999, tag.ID)
	assert.Error(t, err)
}

func TestRepository_GetBooksByTag(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book1 := testutil.SeedBook(t, db, "Tagged Book", "Author", source.ID, user.ID)
	testutil.SeedBook(t, db, "Untagged Book", "Author", source.ID, user.ID)
	tag := testutil.SeedTag(t, db, "fiction", user.ID)

	require.NoError(t, repo.AddTagToBook(book1.ID, tag.ID))

	books, err := repo.GetBooksByTag(tag.ID, user.ID)
	require.NoError(t, err)
	require.Len(t, books, 1)
	assert.Equal(t, "Tagged Book", books[0].Title)
}

func TestRepository_GetBooksByTag_ViaHighlight(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Book With Tagged Highlight", "Author", source.ID, user.ID)
	highlight := testutil.SeedHighlight(t, db, book.ID, "tagged text")
	tag := testutil.SeedTag(t, db, "important", user.ID)

	require.NoError(t, repo.AddTagToHighlight(highlight.ID, tag.ID))

	// Book should appear via highlight tag association
	books, err := repo.GetBooksByTag(tag.ID, user.ID)
	require.NoError(t, err)
	require.Len(t, books, 1)
	assert.Equal(t, "Book With Tagged Highlight", books[0].Title)
}

func TestRepository_GetBooksByTag_NonExistentTag(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetBooksByTag(99999, 1)
	assert.Error(t, err)
}

func TestRepository_GetBooksByTag_FiltersByUser(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user1 := testutil.SeedUser(t, db)
	user2 := &entities.User{Username: "user2", Email: "user2@test.com"}
	require.NoError(t, db.Create(user2).Error)

	book1 := testutil.SeedBook(t, db, "User1 Book", "Author", source.ID, user1.ID)
	book2 := testutil.SeedBook(t, db, "User2 Book", "Author", source.ID, user2.ID)
	tag := testutil.SeedTag(t, db, "shared-name", user1.ID)

	require.NoError(t, repo.AddTagToBook(book1.ID, tag.ID))
	require.NoError(t, repo.AddTagToBook(book2.ID, tag.ID))

	books, err := repo.GetBooksByTag(tag.ID, user1.ID)
	require.NoError(t, err)
	require.Len(t, books, 1)
	assert.Equal(t, "User1 Book", books[0].Title)
}

func TestRepository_GetBookByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "My Book", "Author", source.ID, user.ID)
	testutil.SeedHighlight(t, db, book.ID, "highlight 1")

	found, err := repo.GetBookByID(book.ID)
	require.NoError(t, err)
	assert.Equal(t, "My Book", found.Title)
	assert.Len(t, found.Highlights, 1)
}

func TestRepository_GetBookByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetBookByID(99999)
	assert.Error(t, err)
}

func TestRepository_GetHighlightByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	source := testutil.SeedSource(t, db, "test-source")
	user := testutil.SeedUser(t, db)
	book := testutil.SeedBook(t, db, "Book", "Author", source.ID, user.ID)
	highlight := testutil.SeedHighlight(t, db, book.ID, "some text")

	found, err := repo.GetHighlightByID(highlight.ID)
	require.NoError(t, err)
	assert.Equal(t, "some text", found.Text)
}

func TestRepository_GetHighlightByID_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetHighlightByID(99999)
	assert.Error(t, err)
}
