package moonreader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	dropboxAPIURL      = "https://api.dropboxapi.com/2"
	dropboxContentURL  = "https://content.dropboxapi.com/2"
	defaultDropboxPath = "/Apps/Books/.Moon+/Backup"
)

// DropboxClient handles interactions with Dropbox API
type DropboxClient struct {
	accessToken string
	httpClient  *http.Client
	basePath    string
}

// DropboxFileEntry represents a file entry from Dropbox
type DropboxFileEntry struct {
	Tag            string    `json:".tag"`
	Name           string    `json:"name"`
	PathLower      string    `json:"path_lower"`
	PathDisplay    string    `json:"path_display"`
	ID             string    `json:"id"`
	ClientModified time.Time `json:"client_modified"`
	ServerModified time.Time `json:"server_modified"`
	Size           int64     `json:"size"`
}

// dropboxListFolderResponse represents the response from list_folder API
type dropboxListFolderResponse struct {
	Entries []DropboxFileEntry `json:"entries"`
	Cursor  string             `json:"cursor"`
	HasMore bool               `json:"has_more"`
}

// NewDropboxClient creates a new Dropbox client with the given access token
func NewDropboxClient(accessToken string) *DropboxClient {
	return &DropboxClient{
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		basePath: defaultDropboxPath,
	}
}

// WithBasePath sets a custom base path for MoonReader backups in Dropbox
func (c *DropboxClient) WithBasePath(path string) *DropboxClient {
	c.basePath = path
	return c
}

// ListAllEntries lists all files and folders in the Dropbox path (for debugging)
func (c *DropboxClient) ListAllEntries() ([]DropboxFileEntry, error) {
	return c.listFolder()
}

// ListBackupFiles lists all MoonReader backup files in Dropbox
func (c *DropboxClient) ListBackupFiles() ([]DropboxFileEntry, error) {
	allEntries, err := c.listFolder()
	if err != nil {
		return nil, err
	}

	// Filter for backup files only
	var backupFiles []DropboxFileEntry
	for _, entry := range allEntries {
		if entry.Tag == "file" && isBackupFile(entry.Name) {
			backupFiles = append(backupFiles, entry)
		}
	}

	return backupFiles, nil
}

func (c *DropboxClient) listFolder() ([]DropboxFileEntry, error) {
	var allEntries []DropboxFileEntry

	// Initial request
	requestBody := map[string]any{
		"path":                                c.basePath,
		"recursive":                           false,
		"include_media_info":                  false,
		"include_deleted":                     false,
		"include_has_explicit_shared_members": false,
		"include_mounted_folders":             true,
		"include_non_downloadable_files":      false,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", dropboxAPIURL+"/files/list_folder", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list folder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp dropboxListFolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	allEntries = append(allEntries, listResp.Entries...)

	// Handle pagination
	for listResp.HasMore {
		listResp, err = c.listFolderContinue(listResp.Cursor)
		if err != nil {
			return nil, err
		}
		allEntries = append(allEntries, listResp.Entries...)
	}

	return allEntries, nil
}

func (c *DropboxClient) listFolderContinue(cursor string) (dropboxListFolderResponse, error) {
	requestBody := map[string]string{
		"cursor": cursor,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return dropboxListFolderResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", dropboxAPIURL+"/files/list_folder/continue", bytes.NewReader(bodyBytes))
	if err != nil {
		return dropboxListFolderResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return dropboxListFolderResponse{}, fmt.Errorf("failed to continue listing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return dropboxListFolderResponse{}, fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp dropboxListFolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return dropboxListFolderResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return listResp, nil
}

// FindLatestBackup finds the most recent backup file from Dropbox
func (c *DropboxClient) FindLatestBackup() (*DropboxFileEntry, error) {
	files, err := c.ListBackupFiles()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no backup files found in Dropbox path: %s", c.basePath)
	}

	// Sort by filename (which contains timestamp) descending
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name > files[j].Name
	})

	return &files[0], nil
}

// DownloadFile downloads a file from Dropbox to a local path
func (c *DropboxClient) DownloadFile(dropboxPath, localPath string) error {
	// Create the download request with path in header
	pathArg := map[string]string{
		"path": dropboxPath,
	}
	pathArgBytes, err := json.Marshal(pathArg)
	if err != nil {
		return fmt.Errorf("failed to marshal path arg: %w", err)
	}

	req, err := http.NewRequest("POST", dropboxContentURL+"/files/download", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Dropbox-API-Arg", string(pathArgBytes))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// DownloadLatestBackup downloads the latest backup file to a temporary location
// Returns the path to the downloaded file and the temp directory (caller must clean up)
func (c *DropboxClient) DownloadLatestBackup() (filePath string, tempDir string, modTime time.Time, err error) {
	backup, err := c.FindLatestBackup()
	if err != nil {
		return "", "", time.Time{}, err
	}

	tempDir, err = os.MkdirTemp("", "moonreader-dropbox-*")
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to create temp directory: %w", err)
	}

	localPath := filepath.Join(tempDir, backup.Name)
	if err := c.DownloadFile(backup.PathDisplay, localPath); err != nil {
		os.RemoveAll(tempDir)
		return "", "", time.Time{}, err
	}

	return localPath, tempDir, backup.ServerModified, nil
}

// isBackupFile checks if a filename is a MoonReader backup file
func isBackupFile(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".mrstd") || strings.HasSuffix(lower, ".mrpro")
}

// DropboxBackupExtractor extracts backups from Dropbox
type DropboxBackupExtractor struct {
	client    *DropboxClient
	extractor *BackupExtractor
}

// NewDropboxBackupExtractor creates a new DropboxBackupExtractor
func NewDropboxBackupExtractor(accessToken string) *DropboxBackupExtractor {
	return &DropboxBackupExtractor{
		client:    NewDropboxClient(accessToken),
		extractor: &BackupExtractor{},
	}
}

// WithBasePath sets a custom Dropbox path for backups
func (e *DropboxBackupExtractor) WithBasePath(path string) *DropboxBackupExtractor {
	e.client.WithBasePath(path)
	return e
}

// ExtractLatestDatabase downloads and extracts the latest backup from Dropbox
// Returns the path to the extracted database and a cleanup function
func (e *DropboxBackupExtractor) ExtractLatestDatabase() (dbPath string, cleanup func(), backupTime time.Time, err error) {
	// Download the backup
	backupPath, downloadDir, modTime, err := e.client.DownloadLatestBackup()
	if err != nil {
		return "", nil, time.Time{}, fmt.Errorf("failed to download backup: %w", err)
	}

	// Extract the database
	dbPath, extractDir, err := e.extractor.ExtractDatabase(backupPath)
	if err != nil {
		os.RemoveAll(downloadDir)
		return "", nil, time.Time{}, fmt.Errorf("failed to extract database: %w", err)
	}

	// Cleanup function removes both temp directories
	cleanup = func() {
		os.RemoveAll(downloadDir)
		os.RemoveAll(extractDir)
	}

	return dbPath, cleanup, modTime, nil
}
