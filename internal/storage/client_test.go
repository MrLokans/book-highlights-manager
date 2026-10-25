package storage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ListRecursive ---

// --- FilterFiles ---

func TestFilterFiles_ByExtension(t *testing.T) {
	files := []FileInfo{
		{Name: "doc.txt"},
		{Name: "img.png"},
		{Name: "notes.txt"},
		{Name: "photo.jpg"},
	}

	txtFiles := FilterFiles(files, func(f FileInfo) bool {
		return strings.HasSuffix(f.Name, ".txt")
	})

	assert.Len(t, txtFiles, 2)
	assert.Equal(t, "doc.txt", txtFiles[0].Name)
	assert.Equal(t, "notes.txt", txtFiles[1].Name)
}

func TestFilterFiles_NoMatches(t *testing.T) {
	files := []FileInfo{{Name: "a.txt"}, {Name: "b.txt"}}
	result := FilterFiles(files, func(_ FileInfo) bool { return false })
	assert.Empty(t, result)
}

func TestFilterFiles_EmptyInput(t *testing.T) {
	result := FilterFiles(nil, func(_ FileInfo) bool { return true })
	assert.Empty(t, result)
}

func TestFilterFiles_AllMatch(t *testing.T) {
	files := []FileInfo{{Name: "a"}, {Name: "b"}}
	result := FilterFiles(files, func(_ FileInfo) bool { return true })
	assert.Len(t, result, 2)
}

// --- FindLatest ---

// --- DownloadToFile ---
