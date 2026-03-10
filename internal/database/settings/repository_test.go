package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrlokans/assistant/internal/testutil"
)

func TestRepository_SetSetting_New(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.SetSetting("theme", "dark")
	require.NoError(t, err)

	setting, err := repo.GetSetting("theme")
	require.NoError(t, err)
	assert.Equal(t, "theme", setting.Key)
	assert.Equal(t, "dark", setting.Value)
}

func TestRepository_SetSetting_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.SetSetting("theme", "light")
	require.NoError(t, err)

	err = repo.SetSetting("theme", "dark")
	require.NoError(t, err)

	setting, err := repo.GetSetting("theme")
	require.NoError(t, err)
	assert.Equal(t, "dark", setting.Value)
}

func TestRepository_GetSetting_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	_, err := repo.GetSetting("nonexistent")

	assert.Error(t, err)
}

func TestRepository_DeleteSetting(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.SetSetting("to-delete", "value")
	require.NoError(t, err)

	err = repo.DeleteSetting("to-delete")
	require.NoError(t, err)

	_, err = repo.GetSetting("to-delete")
	assert.Error(t, err)
}

func TestRepository_DeleteSetting_NonExistent(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewRepository(db)

	err := repo.DeleteSetting("nonexistent")
	assert.NoError(t, err)
}
