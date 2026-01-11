package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditor(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := "./test_audit"
	defer os.RemoveAll(tempDir)

	auditor := NewAuditor(tempDir)

	t.Run("SaveJSON creates audit directory and saves file", func(t *testing.T) {
		testData := map[string]interface{}{
			"test_field": "test_value",
			"number":     42,
			"array":      []string{"item1", "item2"},
		}

		filename, err := auditor.SaveJSON(testData)
		require.NoError(t, err)
		assert.NotEmpty(t, filename)
		assert.Contains(t, filename, ".json")

		// Verify the directory was created
		_, err = os.Stat(tempDir)
		assert.NoError(t, err)

		// Verify the file was created
		filePath := filepath.Join(tempDir, filename)
		_, err = os.Stat(filePath)
		assert.NoError(t, err)

		// Verify the file content
		fileContent, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var savedData map[string]interface{}
		err = json.Unmarshal(fileContent, &savedData)
		require.NoError(t, err)

		assert.Equal(t, testData["test_field"], savedData["test_field"])
		assert.Equal(t, float64(42), savedData["number"]) // JSON unmarshals numbers as float64

		// JSON unmarshals arrays as []interface{}, so we need to convert for comparison
		expectedArray := []interface{}{"item1", "item2"}
		assert.Equal(t, expectedArray, savedData["array"])
	})

	t.Run("SaveJSON generates unique filenames", func(t *testing.T) {
		testData := map[string]string{"key": "value"}

		filename1, err := auditor.SaveJSON(testData)
		require.NoError(t, err)

		filename2, err := auditor.SaveJSON(testData)
		require.NoError(t, err)

		assert.NotEqual(t, filename1, filename2)
	})

	t.Run("SaveJSON handles nil auditor gracefully", func(t *testing.T) {
		var nilAuditor *Auditor
		testData := map[string]string{"key": "value"}

		// This should panic, but let's test that our code handles nil checks
		assert.Panics(t, func() {
			nilAuditor.SaveJSON(testData)
		})
	})
}
