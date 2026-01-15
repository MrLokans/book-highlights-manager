package users

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestDB(t *testing.T) (*Repository, func()) {
	dbPath := "./test_users_" + t.Name() + ".db"

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&entities.User{})
	require.NoError(t, err)

	repo := NewRepository(db)

	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		os.Remove(dbPath)
	}

	return repo, cleanup
}

func TestRepository_CreateUser(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	user, err := repo.CreateUser("testuser", "test@example.com")

	require.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.NotEmpty(t, user.Token) // Token should be auto-generated
	assert.Len(t, user.Token, 64)  // 32 bytes hex encoded = 64 chars
}

func TestRepository_GetUserByToken(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := repo.CreateUser("testuser", "test@example.com")
	require.NoError(t, err)

	user, err := repo.GetUserByToken(created.Token)

	require.NoError(t, err)
	assert.Equal(t, created.ID, user.ID)
	assert.Equal(t, "testuser", user.Username)
}

func TestRepository_GetUserByToken_NotFound(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := repo.GetUserByToken("nonexistent-token")

	assert.Error(t, err)
}

func TestRepository_GetUserByID(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := repo.CreateUser("testuser", "test@example.com")
	require.NoError(t, err)

	user, err := repo.GetUserByID(created.ID)

	require.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)
}

func TestRepository_GetUserByID_NotFound(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := repo.GetUserByID(999)

	assert.Error(t, err)
}

func TestRepository_GetUserByUsername(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	created, err := repo.CreateUser("testuser", "test@example.com")
	require.NoError(t, err)

	user, err := repo.GetUserByUsername("testuser")

	require.NoError(t, err)
	assert.Equal(t, created.ID, user.ID)
}

func TestRepository_GetUserByUsername_NotFound(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := repo.GetUserByUsername("nonexistent")

	assert.Error(t, err)
}

func TestRepository_CreateUser_UniqueTokens(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	user1, err := repo.CreateUser("user1", "user1@example.com")
	require.NoError(t, err)

	user2, err := repo.CreateUser("user2", "user2@example.com")
	require.NoError(t, err)

	// Tokens should be unique
	assert.NotEqual(t, user1.Token, user2.Token)
}
