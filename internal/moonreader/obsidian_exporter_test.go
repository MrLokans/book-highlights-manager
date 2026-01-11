package moonreader

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObsidianRenderer_GetCalloutType(t *testing.T) {
	renderer := NewObsidianRenderer()

	tests := []struct {
		name          string
		note          *LocalNote
		expectedType  string
	}{
		{
			name: "strikethrough -> failure",
			note: &LocalNote{
				Strikethrough: true,
				Color:         "-256",
			},
			expectedType: "failure",
		},
		{
			name: "underline -> success",
			note: &LocalNote{
				Underline: true,
				Color:     "-256",
			},
			expectedType: "success",
		},
		{
			name: "yellow -> quote",
			note: &LocalNote{
				Color: "-256", // Yellow
			},
			expectedType: "quote",
		},
		{
			name: "unknown color -> quote",
			note: &LocalNote{
				Color: "-12345678",
			},
			expectedType: "quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderer.GetCalloutType(tt.note)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestObsidianRenderer_FormatRelativeTime(t *testing.T) {
	renderer := NewObsidianRenderer()
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		contains string
	}{
		{
			name:     "just now",
			time:     now,
			contains: "just now",
		},
		{
			name:     "minutes ago",
			time:     now.Add(-30 * time.Minute),
			contains: "minutes ago",
		},
		{
			name:     "hours ago",
			time:     now.Add(-3 * time.Hour),
			contains: "hours ago",
		},
		{
			name:     "yesterday",
			time:     now.Add(-26 * time.Hour),
			contains: "yesterday",
		},
		{
			name:     "days ago",
			time:     now.Add(-3 * 24 * time.Hour),
			contains: "days ago",
		},
		{
			name:     "weeks ago",
			time:     now.Add(-14 * 24 * time.Hour),
			contains: "weeks ago",
		},
		{
			name:     "old date shows month",
			time:     now.Add(-60 * 24 * time.Hour),
			contains: ",", // Contains comma like "January 02, 2006"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderer.FormatRelativeTime(tt.time)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestObsidianRenderer_RenderNote(t *testing.T) {
	renderer := NewObsidianRenderer()
	now := time.Now()

	note := &LocalNote{
		Original:  "This is the highlighted text.",
		Bookmark:  "Page 42",
		Time:      now,
		Color:     "-256",
		Underline: true,
	}

	result := renderer.RenderNote(note)

	// Check callout syntax
	assert.Contains(t, result, "> [!success]")
	// Check bookmark
	assert.Contains(t, result, "Page 42")
	// Check text
	assert.Contains(t, result, "This is the highlighted text.")
	// Check underline indicator
	assert.Contains(t, result, "📝 underlined")
}

func TestObsidianRenderer_CalculateReadingStats(t *testing.T) {
	renderer := NewObsidianRenderer()
	now := time.Now()

	notes := []*LocalNote{
		{Time: now.Add(-10 * 24 * time.Hour), Underline: true, Color: "-256"},
		{Time: now.Add(-5 * 24 * time.Hour), Strikethrough: true, Color: "-256"},
		{Time: now, Color: "-16711936"}, // Green
	}

	stats := renderer.CalculateReadingStats(notes)

	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalHighlights)
	assert.Equal(t, 10, stats.ReadingSpanDays)
	assert.Equal(t, 1, stats.UnderlinedCount)
	assert.Equal(t, 1, stats.StrikethroughCount)
}

func TestObsidianRenderer_RenderBook(t *testing.T) {
	renderer := NewObsidianRenderer()
	now := time.Now()

	notes := []*LocalNote{
		{
			BookTitle: "Test Book",
			Filename:  "/path/Test Book - Test Author.epub",
			Original:  "First highlight",
			Time:      now.Add(-24 * time.Hour),
			Color:     "-256",
		},
		{
			BookTitle: "Test Book",
			Filename:  "/path/Test Book - Test Author.epub",
			Original:  "Second highlight",
			Time:      now,
			Color:     "-256",
		},
	}

	book := NewBookContainer("Test Book", notes)
	result := renderer.RenderBook(book)

	// Check frontmatter
	assert.Contains(t, result, "---")
	assert.Contains(t, result, "book_title: Test Book")
	assert.Contains(t, result, "book_author: Test Author")
	assert.Contains(t, result, "content_source: moonreader_import")
	assert.Contains(t, result, "content_type: book_highlights")
	assert.Contains(t, result, "highlights_count: 2")

	// Check book header
	assert.Contains(t, result, "# Test Book")
	assert.Contains(t, result, "*by Test Author*")

	// Check reading summary
	assert.Contains(t, result, "[!info] Reading Summary")
	assert.Contains(t, result, "**2 highlights**")

	// Check highlights
	assert.Contains(t, result, "First highlight")
	assert.Contains(t, result, "Second highlight")
}

func TestObsidianExporter_Export(t *testing.T) {
	// Setup temp directories
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	outputDir := filepath.Join(tempDir, "output")

	// Create accessor and add notes
	accessor, err := NewLocalDBAccessor(dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	notes := []*MoonReaderNote{
		{
			ID:             1,
			BookTitle:      "Book One",
			Filename:       "/path/Book One - Author A.epub",
			HighlightColor: "-256",
			TimeMs:         time.Now().Add(-24 * time.Hour).UnixMilli(),
			Original:       "Highlight from book one",
		},
		{
			ID:             2,
			BookTitle:      "Book Two",
			Filename:       "/path/Book Two - Author B.epub",
			HighlightColor: "-256",
			TimeMs:         time.Now().UnixMilli(),
			Original:       "Highlight from book two",
		},
	}
	err = accessor.UpsertNotes(notes)
	require.NoError(t, err)

	// Create exporter and export
	exporter := NewObsidianExporter(outputDir, accessor)
	result, err := exporter.Export()
	require.NoError(t, err)

	// Verify results
	assert.Len(t, result.ExportedFiles, 2)
	assert.Empty(t, result.Errors)

	// Verify files exist
	for _, path := range result.ExportedFiles {
		_, err := os.Stat(path)
		assert.NoError(t, err)
	}

	// Verify content of one file
	bookOnePath := result.ExportedFiles["Book One"]
	content, err := os.ReadFile(bookOnePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Book One")
	assert.Contains(t, string(content), "Author A")
	assert.Contains(t, string(content), "Highlight from book one")

	// Verify files are in moonreader subdirectory
	assert.Contains(t, bookOnePath, "/moonreader/")
}

func TestObsidianExporter_ExportSingleBook(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	outputDir := filepath.Join(tempDir, "output")

	accessor, err := NewLocalDBAccessor(dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	notes := []*MoonReaderNote{
		{
			ID:             1,
			BookTitle:      "Target Book",
			Filename:       "/path/Target Book - Author.epub",
			HighlightColor: "-256",
			TimeMs:         time.Now().UnixMilli(),
			Original:       "Target highlight",
		},
		{
			ID:             2,
			BookTitle:      "Other Book",
			Filename:       "/path/Other Book.epub",
			HighlightColor: "-256",
			TimeMs:         time.Now().UnixMilli(),
			Original:       "Other highlight",
		},
	}
	err = accessor.UpsertNotes(notes)
	require.NoError(t, err)

	exporter := NewObsidianExporter(outputDir, accessor)

	// Export single book
	path, err := exporter.ExportSingleBook("Target Book")
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	assert.NoError(t, err)

	// Verify content
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Target Book")
	assert.Contains(t, string(content), "Target highlight")
	assert.NotContains(t, string(content), "Other highlight")

	// Verify moonreader subdirectory was created with one file
	moonreaderDir := filepath.Join(outputDir, "moonreader")
	files, err := os.ReadDir(moonreaderDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)

	// Verify file is in moonreader subdirectory
	assert.Contains(t, path, "/moonreader/")
}

func TestObsidianExporter_OverwritesExistingFiles(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	outputDir := filepath.Join(tempDir, "output")

	accessor, err := NewLocalDBAccessor(dbPath)
	require.NoError(t, err)
	defer accessor.Close()

	notes := []*MoonReaderNote{
		{
			ID:             1,
			BookTitle:      "Test Book",
			Filename:       "/path/Test Book.epub",
			HighlightColor: "-256",
			TimeMs:         time.Now().UnixMilli(),
			Original:       "Updated highlight",
		},
	}
	err = accessor.UpsertNotes(notes)
	require.NoError(t, err)

	exporter := NewObsidianExporter(outputDir, accessor)

	// Create moonreader subdirectory and an existing file with old content
	moonreaderDir := filepath.Join(outputDir, "moonreader")
	err = os.MkdirAll(moonreaderDir, 0755)
	require.NoError(t, err)
	existingFilePath := filepath.Join(moonreaderDir, "Test Book.md")
	err = os.WriteFile(existingFilePath, []byte("old content"), 0644)
	require.NoError(t, err)

	// Export should overwrite the existing file
	result, err := exporter.Export()
	require.NoError(t, err)

	path := result.ExportedFiles["Test Book"]
	assert.Equal(t, existingFilePath, path)

	// Verify file was overwritten with new content
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Updated highlight")
	assert.NotContains(t, string(content), "old content")

	// Only one file should exist in moonreader dir (no duplicates created)
	files, err := os.ReadDir(moonreaderDir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}
