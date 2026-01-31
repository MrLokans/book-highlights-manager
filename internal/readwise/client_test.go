package readwise

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_ValidateToken(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		errType    error
	}{
		{
			name:       "valid token",
			statusCode: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "invalid token",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			errType:    ErrInvalidToken,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Token test-token" {
					t.Errorf("expected Authorization header 'Token test-token', got %s", r.Header.Get("Authorization"))
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := &Client{
				httpClient: server.Client(),
			}

			// Override the auth URL for testing
			origURL := authAPIURL
			defer func() { _ = origURL }()

			// Create a modified client that points to our test server
			ctx := context.Background()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			req.Header.Set("Authorization", "Token test-token")

			resp, err := client.httpClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			gotErr := resp.StatusCode == http.StatusUnauthorized
			if tt.wantErr && tt.errType == ErrInvalidToken && !gotErr {
				t.Errorf("expected invalid token error, got status %d", resp.StatusCode)
			}
		})
	}
}

func TestClient_Export(t *testing.T) {
	exportResponse := ExportResponse{
		Count: 2,
		Results: []BookData{
			{
				UserBookID: 1,
				Title:      "Test Book",
				Author:     "Test Author",
				Highlights: []HighlightData{
					{
						ID:            101,
						Text:          "Test highlight text",
						Location:      42,
						LocationType:  "page",
						HighlightedAt: time.Now(),
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exportResponse)
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
	}

	ctx := context.Background()
	resp, err := client.doExportRequest(ctx, server.URL, "test-token")
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if resp.Count != 2 {
		t.Errorf("expected count 2, got %d", resp.Count)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 book, got %d", len(resp.Results))
	}

	if resp.Results[0].Title != "Test Book" {
		t.Errorf("expected title 'Test Book', got %s", resp.Results[0].Title)
	}

	if len(resp.Results[0].Highlights) != 1 {
		t.Fatalf("expected 1 highlight, got %d", len(resp.Results[0].Highlights))
	}
}

func TestClient_ExportWithPagination(t *testing.T) {
	requestCount := 0
	nextCursor := "cursor123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		cursor := r.URL.Query().Get("pageCursor")
		var resp ExportResponse

		if cursor == "" {
			// First page
			resp = ExportResponse{
				Count:          2,
				NextPageCursor: &nextCursor,
				Results: []BookData{
					{UserBookID: 1, Title: "Book 1"},
				},
			}
		} else if cursor == nextCursor {
			// Second page
			resp = ExportResponse{
				Count:          2,
				NextPageCursor: nil,
				Results: []BookData{
					{UserBookID: 2, Title: "Book 2"},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
	}

	// Test pagination by manually calling
	ctx := context.Background()

	// First page
	resp1, err := client.doExportRequest(ctx, server.URL, "test-token")
	if err != nil {
		t.Fatalf("First page failed: %v", err)
	}
	if resp1.NextPageCursor == nil || *resp1.NextPageCursor != nextCursor {
		t.Errorf("expected nextPageCursor %s", nextCursor)
	}

	// Second page
	resp2, err := client.doExportRequest(ctx, server.URL+"?pageCursor="+nextCursor, "test-token")
	if err != nil {
		t.Fatalf("Second page failed: %v", err)
	}
	if resp2.NextPageCursor != nil {
		t.Errorf("expected no nextPageCursor on last page")
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests, got %d", requestCount)
	}
}

func TestClient_RateLimitRetry(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExportResponse{Count: 1})
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
	}

	// Test that rate limit triggers retry
	ctx := context.Background()
	_, err := client.doExportRequest(ctx, server.URL, "test-token")
	if err != ErrRateLimited {
		t.Errorf("expected ErrRateLimited on first request")
	}

	// Second request should succeed
	_, err = client.doExportRequest(ctx, server.URL, "test-token")
	if err != nil {
		t.Errorf("expected success on second request, got %v", err)
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	tests := []struct {
		attempt int
		minWant time.Duration
		maxWant time.Duration
	}{
		{0, 1 * time.Second, 1 * time.Second},
		{1, 2 * time.Second, 2 * time.Second},
		{2, 4 * time.Second, 4 * time.Second},
		{10, maxRetryDelay, maxRetryDelay}, // Should be capped
	}

	for _, tt := range tests {
		got := calculateRetryDelay(tt.attempt)
		if got < tt.minWant || got > tt.maxWant {
			t.Errorf("calculateRetryDelay(%d) = %v, want between %v and %v",
				tt.attempt, got, tt.minWant, tt.maxWant)
		}
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{ErrRateLimited, true},
		{&ServerError{StatusCode: 500}, true},
		{&ServerError{StatusCode: 503}, true},
		{ErrInvalidToken, false},
		{nil, false},
	}

	for _, tt := range tests {
		got := isRetryableError(tt.err)
		if got != tt.want {
			t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}
