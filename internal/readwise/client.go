package readwise

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	exportAPIURL = "https://readwise.io/api/v2/export/"
	authAPIURL   = "https://readwise.io/api/v2/auth/"

	defaultTimeout     = 30 * time.Second
	maxRetries         = 3
	initialRetryDelay  = 1 * time.Second
	maxRetryDelay      = 30 * time.Second
	retryBackoffFactor = 2
)

// Client interfaces with the Readwise Export API
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Readwise API client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// ExportResponse represents the response from the Readwise Export API
type ExportResponse struct {
	Count          int        `json:"count"`
	NextPageCursor *string    `json:"nextPageCursor"`
	Results        []BookData `json:"results"`
}

// BookData represents a book from the Readwise Export API
type BookData struct {
	UserBookID    int             `json:"user_book_id"`
	Title         string          `json:"title"`
	Author        string          `json:"author"`
	ReadableTitle string          `json:"readable_title"`
	Source        string          `json:"source"`
	CoverImageURL string          `json:"cover_image_url"`
	UniqueURL     string          `json:"unique_url"`
	BookTags      []TagData       `json:"book_tags"`
	Category      string          `json:"category"`
	DocumentNote  string          `json:"document_note"`
	ReadwiseURL   string          `json:"readwise_url"`
	SourceURL     string          `json:"source_url"`
	ASIN          string          `json:"asin"`
	Highlights    []HighlightData `json:"highlights"`
}

// HighlightData represents a highlight from the Readwise Export API
type HighlightData struct {
	ID            int       `json:"id"`
	Text          string    `json:"text"`
	Location      int       `json:"location"`
	LocationType  string    `json:"location_type"`
	Note          string    `json:"note"`
	Color         string    `json:"color"`
	HighlightedAt time.Time `json:"highlighted_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ExternalID    string    `json:"external_id"`
	EndLocation   int       `json:"end_location"`
	URL           *string   `json:"url"`
	BookID        int       `json:"book_id"`
	Tags          []TagData `json:"tags"`
	IsFavorite    bool      `json:"is_favorite"`
	IsDiscarded   bool      `json:"is_discard"`
	ReadwiseURL   string    `json:"readwise_url"`
}

// TagData represents a tag from the Readwise Export API
type TagData struct {
	Name string `json:"name"`
}

// ValidateToken checks if a token is valid by calling the auth endpoint
func (c *Client) ValidateToken(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authAPIURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrInvalidToken
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Export fetches highlights from the Readwise Export API with optional pagination and incremental sync
func (c *Client) Export(ctx context.Context, token string, updatedAfter *time.Time, cursor string) (*ExportResponse, error) {
	u, err := url.Parse(exportAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	if updatedAfter != nil {
		q.Set("updatedAfter", updatedAfter.Format(time.RFC3339))
	}
	if cursor != "" {
		q.Set("pageCursor", cursor)
	}
	u.RawQuery = q.Encode()

	var resp *ExportResponse
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := calculateRetryDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, lastErr = c.doExportRequest(ctx, u.String(), token)
		if lastErr == nil {
			return resp, nil
		}

		// Only retry on rate limits or server errors
		if !isRetryableError(lastErr) {
			return nil, lastErr
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// ExportAll fetches all highlights by paginating through all pages
func (c *Client) ExportAll(ctx context.Context, token string, updatedAfter *time.Time) ([]BookData, error) {
	var allBooks []BookData
	var cursor string

	for {
		resp, err := c.Export(ctx, token, updatedAfter, cursor)
		if err != nil {
			return nil, err
		}

		allBooks = append(allBooks, resp.Results...)

		if resp.NextPageCursor == nil || *resp.NextPageCursor == "" {
			break
		}
		cursor = *resp.NextPageCursor
	}

	return allBooks, nil
}

func (c *Client) doExportRequest(ctx context.Context, url, token string) (*ExportResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Token "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrInvalidToken
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode >= 500 {
		return nil, &ServerError{StatusCode: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var exportResp ExportResponse
	if err := json.NewDecoder(resp.Body).Decode(&exportResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &exportResp, nil
}

func calculateRetryDelay(attempt int) time.Duration {
	delay := initialRetryDelay
	for i := 0; i < attempt; i++ {
		delay *= time.Duration(retryBackoffFactor)
	}
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	return delay
}

func isRetryableError(err error) bool {
	if err == ErrRateLimited {
		return true
	}
	if _, ok := err.(*ServerError); ok {
		return true
	}
	return false
}
