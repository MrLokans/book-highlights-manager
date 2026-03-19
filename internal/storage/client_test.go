package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient implements Client for testing helper functions.
type mockClient struct {
	listFunc     func(ctx context.Context, path string) ([]FileInfo, error)
	downloadFunc func(ctx context.Context, path string) (io.ReadCloser, error)
}

func (m *mockClient) List(ctx context.Context, path string) ([]FileInfo, error) {
	return m.listFunc(ctx, path)
}

func (m *mockClient) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	if m.downloadFunc != nil {
		return m.downloadFunc(ctx, path)
	}
	return io.NopCloser(strings.NewReader("content")), nil
}

func (m *mockClient) Upload(_ context.Context, _ string, _ io.Reader) error { return nil }
func (m *mockClient) Delete(_ context.Context, _ string) error              { return nil }
func (m *mockClient) Exists(_ context.Context, _ string) (bool, error)      { return false, nil }
func (m *mockClient) GetMetadata(_ context.Context, _ string) (*FileInfo, error) {
	return nil, nil
}

// --- ListRecursive ---

func TestListRecursive_FlatDirectory(t *testing.T) {
	client := &mockClient{
		listFunc: func(_ context.Context, _ string) ([]FileInfo, error) {
			return []FileInfo{
				{Name: "file1.txt", Path: "/file1.txt"},
				{Name: "file2.txt", Path: "/file2.txt"},
			}, nil
		},
	}

	files, err := ListRecursive(context.Background(), client, "/")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestListRecursive_NestedDirectories(t *testing.T) {
	client := &mockClient{
		listFunc: func(_ context.Context, path string) ([]FileInfo, error) {
			switch path {
			case "/":
				return []FileInfo{
					{Name: "sub", Path: "/sub", IsDir: true},
					{Name: "root.txt", Path: "/root.txt"},
				}, nil
			case "/sub":
				return []FileInfo{
					{Name: "nested.txt", Path: "/sub/nested.txt"},
				}, nil
			default:
				return nil, nil
			}
		},
	}

	files, err := ListRecursive(context.Background(), client, "/")
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Equal(t, "/sub/nested.txt", files[0].Path)
	assert.Equal(t, "/root.txt", files[1].Path)
}

func TestListRecursive_EmptyDirectory(t *testing.T) {
	client := &mockClient{
		listFunc: func(_ context.Context, _ string) ([]FileInfo, error) {
			return nil, nil
		},
	}

	files, err := ListRecursive(context.Background(), client, "/empty")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestListRecursive_ErrorOnRoot(t *testing.T) {
	client := &mockClient{
		listFunc: func(_ context.Context, _ string) ([]FileInfo, error) {
			return nil, errors.New("access denied")
		},
	}

	_, err := ListRecursive(context.Background(), client, "/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestListRecursive_ErrorOnSubdirectory(t *testing.T) {
	client := &mockClient{
		listFunc: func(_ context.Context, path string) ([]FileInfo, error) {
			if path == "/" {
				return []FileInfo{{Name: "sub", Path: "/sub", IsDir: true}}, nil
			}
			return nil, errors.New("sub error")
		},
	}

	_, err := ListRecursive(context.Background(), client, "/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sub error")
}

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

func TestFindLatest_MultipleFiles(t *testing.T) {
	now := time.Now()
	files := []FileInfo{
		{Name: "old", ModifiedAt: now.Add(-2 * time.Hour)},
		{Name: "newest", ModifiedAt: now},
		{Name: "middle", ModifiedAt: now.Add(-1 * time.Hour)},
	}

	latest := FindLatest(files)
	require.NotNil(t, latest)
	assert.Equal(t, "newest", latest.Name)
}

func TestFindLatest_SingleFile(t *testing.T) {
	files := []FileInfo{{Name: "only", ModifiedAt: time.Now()}}
	latest := FindLatest(files)
	require.NotNil(t, latest)
	assert.Equal(t, "only", latest.Name)
}

func TestFindLatest_EmptySlice(t *testing.T) {
	latest := FindLatest(nil)
	assert.Nil(t, latest)
}

func TestFindLatest_SameTimestamp(t *testing.T) {
	now := time.Now()
	files := []FileInfo{
		{Name: "first", ModifiedAt: now},
		{Name: "second", ModifiedAt: now},
	}

	latest := FindLatest(files)
	require.NotNil(t, latest)
	// Should return first since none is "After" the other
	assert.Equal(t, "first", latest.Name)
}

// --- DownloadToFile ---

func TestDownloadToFile_CallsDownload(t *testing.T) {
	downloaded := false
	client := &mockClient{
		downloadFunc: func(_ context.Context, path string) (io.ReadCloser, error) {
			assert.Equal(t, "/remote/file.txt", path)
			downloaded = true
			return io.NopCloser(strings.NewReader("data")), nil
		},
	}

	// DownloadToFile currently has a no-op writeToFile, so it won't error on write
	err := DownloadToFile(context.Background(), client, "/remote/file.txt", "/tmp/test")
	require.NoError(t, err)
	assert.True(t, downloaded)
}

func TestDownloadToFile_DownloadError(t *testing.T) {
	client := &mockClient{
		downloadFunc: func(_ context.Context, _ string) (io.ReadCloser, error) {
			return nil, errors.New("network error")
		},
	}

	err := DownloadToFile(context.Background(), client, "/remote/file.txt", "/tmp/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}
