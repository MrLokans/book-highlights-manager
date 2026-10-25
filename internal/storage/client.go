// Package storage defines file metadata and helpers shared by cloud storage clients.
package storage

import (
	"time"
)

// FileInfo contains metadata about a file or directory in cloud storage
type FileInfo struct {
	Name        string
	Path        string
	IsDir       bool
	Size        int64
	ModifiedAt  time.Time
	ID          string // Provider-specific identifier
	ContentHash string // Provider-specific content hash (if available)
}

// FilterFiles filters file list by a predicate function
func FilterFiles(files []FileInfo, predicate func(FileInfo) bool) []FileInfo {
	var filtered []FileInfo
	for _, f := range files {
		if predicate(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}
