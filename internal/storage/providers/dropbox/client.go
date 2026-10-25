// Package dropbox is a Dropbox API client for listing and downloading files.
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

// APIError represents a Dropbox API error response with HTTP status code
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("dropbox API error (status %d): %s", e.StatusCode, e.Body)
}

const (
	defaultAPIURL     = "https://api.dropboxapi.com/2"
	defaultContentURL = "https://content.dropboxapi.com/2"
)

// Client lists and downloads Dropbox files using an auto-refreshing token source.
type Client struct {
	tokenSource oauth2.TokenSource
	httpClient  *http.Client
	apiURL      string
	contentURL  string
}

// NewClient creates a new Dropbox storage client
func NewClient(tokenSource oauth2.TokenSource) *Client {
	return &Client{
		tokenSource: tokenSource,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		apiURL:     defaultAPIURL,
		contentURL: defaultContentURL,
	}
}

// Option configures a Client.
type Option func(*Client)

// WithURLs overrides the API and content URLs (for testing).
func WithURLs(apiURL, contentURL string) Option {
	return func(c *Client) {
		c.apiURL = apiURL
		c.contentURL = contentURL
	}
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// NewClientWithOptions creates a Dropbox client with the given options.
func NewClientWithOptions(tokenSource oauth2.TokenSource, opts ...Option) *Client {
	c := NewClient(tokenSource)
	for _, opt := range opts {
		opt(c)
	}
	return c
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

// List returns entries in the specified Dropbox directory.
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/files/list_folder", bytes.NewReader(bodyBytes))
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.apiURL+"/files/list_folder/continue", bytes.NewReader(bodyBytes))
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

// Download retrieves the contents of a file from Dropbox.
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.contentURL+"/files/download", nil)
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
		_ = resp.Body.Close() //nolint:gosec // error path, body already read
		return nil, fmt.Errorf("dropbox API error (status %d): %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}
