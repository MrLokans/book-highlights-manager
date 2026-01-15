package settings

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
	dbPath := "./test_settings_" + t.Name() + ".db"

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&entities.Setting{})
	require.NoError(t, err)

	repo := NewRepository(db)

	cleanup := func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		os.Remove(dbPath)
	}

	return repo, cleanup
}

func TestRepository_SetSetting_New(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.SetSetting("theme", "dark")
	require.NoError(t, err)

	setting, err := repo.GetSetting("theme")
	require.NoError(t, err)
	assert.Equal(t, "theme", setting.Key)
	assert.Equal(t, "dark", setting.Value)
}

func TestRepository_SetSetting_Update(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Set initial value
	err := repo.SetSetting("theme", "light")
	require.NoError(t, err)

	// Update value
	err = repo.SetSetting("theme", "dark")
	require.NoError(t, err)

	setting, err := repo.GetSetting("theme")
	require.NoError(t, err)
	assert.Equal(t, "dark", setting.Value)
}

func TestRepository_GetSetting_NotFound(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := repo.GetSetting("nonexistent")

	assert.Error(t, err)
}

func TestRepository_DeleteSetting(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	err := repo.SetSetting("to-delete", "value")
	require.NoError(t, err)

	err = repo.DeleteSetting("to-delete")
	require.NoError(t, err)

	_, err = repo.GetSetting("to-delete")
	assert.Error(t, err)
}

func TestRepository_DeleteSetting_NonExistent(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	// Should not error even if key doesn't exist
	err := repo.DeleteSetting("nonexistent")
	assert.NoError(t, err)
}
