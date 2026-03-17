package settingsstore

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSetting = Setting{
	Key:     "test_setting",
	EnvVars: []string{"TEST_SETTING_PRIMARY", "TEST_SETTING_FALLBACK"},
	Default: "default_value",
}

var testBoolSetting = Setting{
	Key:     "test_bool_setting",
	EnvVars: []string{"TEST_BOOL_SETTING"},
	Default: "false",
}

func TestGet_Default(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	assert.Equal(t, "default_value", store.Get(testSetting))
}

func TestGet_Environment(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	os.Setenv("TEST_SETTING_PRIMARY", "env_value")
	defer os.Unsetenv("TEST_SETTING_PRIMARY")

	assert.Equal(t, "env_value", store.Get(testSetting))
}

func TestGet_EnvironmentFallback(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Only set the fallback env var
	os.Setenv("TEST_SETTING_FALLBACK", "fallback_value")
	defer os.Unsetenv("TEST_SETTING_FALLBACK")

	assert.Equal(t, "fallback_value", store.Get(testSetting))
}

func TestGet_EnvironmentPrimaryWinsOverFallback(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	os.Setenv("TEST_SETTING_PRIMARY", "primary")
	os.Setenv("TEST_SETTING_FALLBACK", "fallback")
	defer os.Unsetenv("TEST_SETTING_PRIMARY")
	defer os.Unsetenv("TEST_SETTING_FALLBACK")

	assert.Equal(t, "primary", store.Get(testSetting))
}

func TestGet_DatabaseOverridesEnvironment(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	os.Setenv("TEST_SETTING_PRIMARY", "env_value")
	defer os.Unsetenv("TEST_SETTING_PRIMARY")

	require.NoError(t, store.Set(testSetting, "db_value"))

	assert.Equal(t, "db_value", store.Get(testSetting))
}

func TestGetSource(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Default
	assert.Equal(t, "default", store.GetSource(testSetting))

	// Environment
	os.Setenv("TEST_SETTING_FALLBACK", "val")
	defer os.Unsetenv("TEST_SETTING_FALLBACK")
	assert.Equal(t, "environment", store.GetSource(testSetting))

	// Database
	require.NoError(t, store.Set(testSetting, "db"))
	assert.Equal(t, "database", store.GetSource(testSetting))
}

func TestGetBool(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	// Default false
	assert.False(t, store.GetBool(testBoolSetting))

	// "true"
	require.NoError(t, store.Set(testBoolSetting, "true"))
	assert.True(t, store.GetBool(testBoolSetting))

	// "1"
	require.NoError(t, store.Set(testBoolSetting, "1"))
	assert.True(t, store.GetBool(testBoolSetting))

	// "false"
	require.NoError(t, store.Set(testBoolSetting, "false"))
	assert.False(t, store.GetBool(testBoolSetting))

	// Env "true"
	require.NoError(t, db.DeleteSetting(testBoolSetting.Key))
	os.Setenv("TEST_BOOL_SETTING", "true")
	defer os.Unsetenv("TEST_BOOL_SETTING")
	assert.True(t, store.GetBool(testBoolSetting))
}

func TestSetBool(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	require.NoError(t, store.SetBool(testBoolSetting, true))
	assert.True(t, store.GetBool(testBoolSetting))

	require.NoError(t, store.SetBool(testBoolSetting, false))
	assert.False(t, store.GetBool(testBoolSetting))
}

func TestClear(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	require.NoError(t, store.Set(testSetting, "db_value"))
	assert.Equal(t, "db_value", store.Get(testSetting))

	require.NoError(t, store.Clear(testSetting))
	assert.Equal(t, "default_value", store.Get(testSetting))
}

func TestClear_Multiple(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	require.NoError(t, store.Set(testSetting, "val1"))
	require.NoError(t, store.SetBool(testBoolSetting, true))

	require.NoError(t, store.Clear(testSetting, testBoolSetting))

	assert.Equal(t, "default_value", store.Get(testSetting))
	assert.False(t, store.GetBool(testBoolSetting))
}

func TestGet_NoEnvVars(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	store := New(db)

	noEnvSetting := Setting{Key: "no_env", Default: "fallback"}

	assert.Equal(t, "fallback", store.Get(noEnvSetting))
	assert.Equal(t, "default", store.GetSource(noEnvSetting))
}
