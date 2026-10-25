package moonreader

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mrlokans/assistant/internal/storage"
	"github.com/mrlokans/assistant/internal/storage/providers/dropbox"
)

// IsBackupFile reports whether f is a Moon+ Reader backup (.mrstd or .mrpro).
func IsBackupFile(f storage.FileInfo) bool {
	if f.IsDir {
		return false
	}
	lower := strings.ToLower(f.Name)
	return strings.HasSuffix(lower, ".mrstd") || strings.HasSuffix(lower, ".mrpro")
}

// ExtractLatestDropboxBackup downloads the newest Moon+ Reader backup in dir and extracts its notes database.
// The caller must call cleanup once done with dbPath.
func ExtractLatestDropboxBackup(ctx context.Context, client *dropbox.Client, dir string) (dbPath string, backup storage.FileInfo, cleanup func(), err error) {
	entries, err := client.List(ctx, dir)
	if err != nil {
		return "", backup, nil, fmt.Errorf("failed to list Dropbox folder: %w", err)
	}
	backups := storage.FilterFiles(entries, IsBackupFile)
	if len(backups) == 0 {
		return "", backup, nil, fmt.Errorf("no backup files found in Dropbox path: %s", dir)
	}
	// Moon+ names backups by timestamp, so the greatest name is the newest backup.
	backup = slices.MaxFunc(backups, func(a, b storage.FileInfo) int { return cmp.Compare(a.Name, b.Name) })

	downloadDir, err := os.MkdirTemp("", "moonreader-dropbox-*")
	if err != nil {
		return "", backup, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	localPath := filepath.Join(downloadDir, filepath.Base(backup.Name))
	if err := downloadTo(ctx, client, backup.Path, localPath); err != nil {
		_ = os.RemoveAll(downloadDir) //nolint:gosec // best-effort cleanup
		return "", backup, nil, err
	}

	dbPath, extractDir, err := (&BackupExtractor{}).ExtractDatabase(localPath)
	if err != nil {
		_ = os.RemoveAll(downloadDir) //nolint:gosec // best-effort cleanup
		return "", backup, nil, fmt.Errorf("failed to extract database: %w", err)
	}

	cleanup = func() {
		_ = os.RemoveAll(downloadDir) //nolint:gosec // best-effort cleanup
		_ = os.RemoveAll(extractDir)  //nolint:gosec // best-effort cleanup
	}
	return dbPath, backup, cleanup, nil
}

func downloadTo(ctx context.Context, client *dropbox.Client, remotePath, localPath string) error {
	reader, err := client.Download(ctx, remotePath)
	if err != nil {
		return fmt.Errorf("failed to download backup: %w", err)
	}
	defer reader.Close()

	file, err := os.Create(filepath.Clean(localPath))
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close() //nolint:gosec // already failing; write error wins
		return fmt.Errorf("failed to write backup: %w", err)
	}
	return file.Close()
}
