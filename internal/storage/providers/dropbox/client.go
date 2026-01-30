package dropbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mrlokans/assistant/internal/oauth2"
	"github.com/mrlokans/assistant/internal/storage"
)

const (
	dropboxAPIURL     = "https://api.dropboxapi.com/2"
	dropboxContentURL = "https://content.dropboxapi.com/2"
)

// Client implements storage.Client for Dropbox
type Client struct {
	tokenSource oauth2.TokenSource
	httpClient  *http.Client
}

// NewClient creates a new Dropbox storage client
func NewClient(tokenSource oauth2.TokenSource) *Client {
	return &Client{
		tokenSource: tokenSource,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// listFolderResponse represents the response from list_folder API
type listFolderResponse struct {
	Entries []struct {
		Tag            string    `json:".tag"`
		Name           string    `json:"name"`
		PathLower      string    `json:"path_lower"`
		PathDisplay    string    `json:"path_display"`
		ID             string    `json:"id"`
		ClientModified time.Time `json:"client_modified"`
		ServerModified time.Time `json:"server_modified"`
		Size           int64     `json:"size"`
		ContentHash    string    `json:"content_hash"`
	} `json:"entries"`
	Cursor  string `json:"cursor"`
	HasMore bool   `json:"has_more"`
}

func (c *Client) List(ctx context.Context, path string) ([]storage.FileInfo, error) {
	token, err := c.tokenSource.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	var allEntries []storage.FileInfo

	// Initial request
	requestBody := map[string]any{
		"path":                                path,
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

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxAPIURL+"/files/list_folder", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
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

	var listResp listFolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	allEntries = append(allEntries, convertEntries(listResp.Entries)...)

	// Handle pagination
	for listResp.HasMore {
		listResp, err = c.listFolderContinue(ctx, token, listResp.Cursor)
		if err != nil {
			return nil, err
		}
		allEntries = append(allEntries, convertEntries(listResp.Entries)...)
	}

	return allEntries, nil
}

func (c *Client) listFolderContinue(ctx context.Context, token, cursor string) (listFolderResponse, error) {
	requestBody := map[string]string{
		"cursor": cursor,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return listFolderResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxAPIURL+"/files/list_folder/continue", bytes.NewReader(bodyBytes))
	if err != nil {
		return listFolderResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return listFolderResponse{}, fmt.Errorf("failed to continue listing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return listFolderResponse{}, fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp listFolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return listFolderResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return listResp, nil
}

func convertEntries(entries []struct {
	Tag            string    `json:".tag"`
	Name           string    `json:"name"`
	PathLower      string    `json:"path_lower"`
	PathDisplay    string    `json:"path_display"`
	ID             string    `json:"id"`
	ClientModified time.Time `json:"client_modified"`
	ServerModified time.Time `json:"server_modified"`
	Size           int64     `json:"size"`
	ContentHash    string    `json:"content_hash"`
}) []storage.FileInfo {
	result := make([]storage.FileInfo, len(entries))
	for i, e := range entries {
		result[i] = storage.FileInfo{
			Name:        e.Name,
			Path:        e.PathDisplay,
			IsDir:       e.Tag == "folder",
			Size:        e.Size,
			ModifiedAt:  e.ServerModified,
			ID:          e.ID,
			ContentHash: e.ContentHash,
		}
	}
	return result
}

func (c *Client) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	token, err := c.tokenSource.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	pathArg := map[string]string{
		"path": path,
	}
	pathArgBytes, err := json.Marshal(pathArg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal path arg: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxContentURL+"/files/download", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Dropbox-API-Arg", string(pathArgBytes))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

func (c *Client) Upload(ctx context.Context, path string, content io.Reader) error {
	token, err := c.tokenSource.Token(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	// Read all content to get size and create request body
	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	uploadArg := map[string]any{
		"path":            path,
		"mode":            "overwrite",
		"autorename":      false,
		"mute":            false,
		"strict_conflict": false,
	}
	uploadArgBytes, err := json.Marshal(uploadArg)
	if err != nil {
		return fmt.Errorf("failed to marshal upload arg: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxContentURL+"/files/upload", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Dropbox-API-Arg", string(uploadArgBytes))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, path string) error {
	token, err := c.tokenSource.Token(ctx)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	requestBody := map[string]string{
		"path": path,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxAPIURL+"/files/delete_v2", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) Exists(ctx context.Context, path string) (bool, error) {
	_, err := c.GetMetadata(ctx, path)
	if err != nil {
		// Check if it's a "not found" error
		// Dropbox returns 409 with path/not_found for non-existent files
		return false, nil
	}
	return true, nil
}

func (c *Client) GetMetadata(ctx context.Context, path string) (*storage.FileInfo, error) {
	token, err := c.tokenSource.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	requestBody := map[string]any{
		"path":                                path,
		"include_media_info":                  false,
		"include_deleted":                     false,
		"include_has_explicit_shared_members": false,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", dropboxAPIURL+"/files/get_metadata", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	var metadata struct {
		Tag            string    `json:".tag"`
		Name           string    `json:"name"`
		PathDisplay    string    `json:"path_display"`
		ID             string    `json:"id"`
		ServerModified time.Time `json:"server_modified"`
		Size           int64     `json:"size"`
		ContentHash    string    `json:"content_hash"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &storage.FileInfo{
		Name:        metadata.Name,
		Path:        metadata.PathDisplay,
		IsDir:       metadata.Tag == "folder",
		Size:        metadata.Size,
		ModifiedAt:  metadata.ServerModified,
		ID:          metadata.ID,
		ContentHash: metadata.ContentHash,
	}, nil
}
