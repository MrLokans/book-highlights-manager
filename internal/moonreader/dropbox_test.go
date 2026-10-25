package moonreader

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrlokans/assistant/internal/storage"
	"github.com/mrlokans/assistant/internal/storage/providers/dropbox"
)

type testTokenSource struct{}

func (testTokenSource) Token(_ context.Context) (string, error) { return "test-token", nil }
func (testTokenSource) ForceRefresh(_ context.Context) error    { return nil }
func (testTokenSource) IsValid() bool                           { return true }
func (testTokenSource) ExpiresAt() *time.Time                   { return nil }
func (testTokenSource) AccountID() string                       { return "test-account" }

func backupZip(t *testing.T, dbContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"com.flyersoft.moonreader/_names.list": "/data/other.file\n/data/" + DBManifestLocator + "\n",
		"com.flyersoft.moonreader/2.tag":       dbContent,
	} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestIsBackupFile(t *testing.T) {
	assert.True(t, IsBackupFile(storage.FileInfo{Name: "20240115_120000.mrpro"}))
	assert.True(t, IsBackupFile(storage.FileInfo{Name: "20240115_120000.MRSTD"}))
	assert.False(t, IsBackupFile(storage.FileInfo{Name: "notes.txt"}))
	assert.False(t, IsBackupFile(storage.FileInfo{Name: "dir.mrpro", IsDir: true}))
}

func TestExtractLatestDropboxBackup(t *testing.T) {
	var downloaded string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files/list_folder":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"entries": []map[string]any{
					{".tag": "file", "name": "20240110_100000.mrstd", "path_display": "/Backup/20240110_100000.mrstd"},
					{".tag": "file", "name": "20240115_120000.mrpro", "path_display": "/Backup/20240115_120000.mrpro"},
					{".tag": "file", "name": "readme.txt", "path_display": "/Backup/readme.txt"},
					{".tag": "folder", "name": "old", "path_display": "/Backup/old"},
				},
			})
		case "/files/download":
			downloaded = r.Header.Get("Dropbox-API-Arg")
			_, _ = w.Write(backupZip(t, "sqlite-bytes"))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	client := dropbox.NewClientWithOptions(testTokenSource{}, dropbox.WithURLs(srv.URL, srv.URL), dropbox.WithHTTPClient(srv.Client()))

	dbPath, backup, cleanup, err := ExtractLatestDropboxBackup(context.Background(), client, "/Backup")
	require.NoError(t, err)

	assert.Equal(t, "20240115_120000.mrpro", backup.Name)
	assert.Contains(t, downloaded, "/Backup/20240115_120000.mrpro")
	content, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, "sqlite-bytes", string(content))

	cleanup()
	_, err = os.Stat(dbPath)
	assert.True(t, os.IsNotExist(err), "cleanup should remove extracted files")
}

func TestExtractLatestDropboxBackup_NoBackups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": []map[string]any{{".tag": "file", "name": "a.txt"}}})
	}))
	defer srv.Close()
	client := dropbox.NewClientWithOptions(testTokenSource{}, dropbox.WithURLs(srv.URL, srv.URL), dropbox.WithHTTPClient(srv.Client()))

	_, _, _, err := ExtractLatestDropboxBackup(context.Background(), client, "/Backup")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no backup files found"))
}
