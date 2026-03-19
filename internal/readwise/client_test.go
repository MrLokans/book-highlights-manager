package readwise

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer creates an httptest.Server and a Client pointed at it.
func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClientWithOptions(
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	return srv, client
}

// --- ValidateToken ---

func TestValidateToken_Success_NoContent(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Token my-token", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))

	err := client.ValidateToken(context.Background(), "my-token")
	assert.NoError(t, err)
}

func TestValidateToken_Success_OK(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	err := client.ValidateToken(context.Background(), "my-token")
	assert.NoError(t, err)
}

func TestValidateToken_Unauthorized(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	err := client.ValidateToken(context.Background(), "bad-token")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_UnexpectedStatus(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))

	err := client.ValidateToken(context.Background(), "my-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 403")
	assert.NotErrorIs(t, err, ErrInvalidToken)
}

func TestValidateToken_ContextCancelled(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.ValidateToken(ctx, "my-token")
	require.Error(t, err)
}

// --- Export (single page, with retries) ---

func TestExport_Success(t *testing.T) {
	want := ExportResponse{
		Count: 1,
		Results: []BookData{
			{UserBookID: 42, Title: "Test Book", Author: "Author"},
		},
	}

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Token tok", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))

	got, err := client.Export(context.Background(), "tok", nil, "")
	require.NoError(t, err)
	assert.Equal(t, 1, got.Count)
	require.Len(t, got.Results, 1)
	assert.Equal(t, "Test Book", got.Results[0].Title)
}

func TestExport_PassesUpdatedAfterAndCursor(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.URL.Query().Get("updatedAfter"))
		assert.Equal(t, "abc", r.URL.Query().Get("pageCursor"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExportResponse{})
	}))

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := client.Export(context.Background(), "tok", &ts, "abc")
	assert.NoError(t, err)
}

func TestExport_Unauthorized(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	_, err := client.Export(context.Background(), "bad", nil, "")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestExport_RetriesOnRateLimit(t *testing.T) {
	// Override retry delay for fast tests
	origInitial := initialRetryDelay
	t.Cleanup(func() { resetRetryDelay(origInitial) })
	overrideRetryDelay(1 * time.Millisecond)

	var calls atomic.Int32
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExportResponse{Count: 99})
	}))

	got, err := client.Export(context.Background(), "tok", nil, "")
	require.NoError(t, err)
	assert.Equal(t, 99, got.Count)
	assert.Equal(t, int32(3), calls.Load())
}

func TestExport_RetriesOnServerError(t *testing.T) {
	origInitial := initialRetryDelay
	t.Cleanup(func() { resetRetryDelay(origInitial) })
	overrideRetryDelay(1 * time.Millisecond)

	var calls atomic.Int32
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExportResponse{Count: 1})
	}))

	got, err := client.Export(context.Background(), "tok", nil, "")
	require.NoError(t, err)
	assert.Equal(t, 1, got.Count)
	assert.Equal(t, int32(2), calls.Load())
}

func TestExport_MaxRetriesExceeded(t *testing.T) {
	origInitial := initialRetryDelay
	t.Cleanup(func() { resetRetryDelay(origInitial) })
	overrideRetryDelay(1 * time.Millisecond)

	var calls atomic.Int32
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	_, err := client.Export(context.Background(), "tok", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded")
	assert.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, int32(maxRetries), calls.Load())
}

func TestExport_NoRetryOnClientError(t *testing.T) {
	var calls atomic.Int32
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))

	_, err := client.Export(context.Background(), "bad", nil, "")
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.Equal(t, int32(1), calls.Load(), "should not retry on 401")
}

func TestExport_ContextCancelledDuringRetry(t *testing.T) {
	origInitial := initialRetryDelay
	t.Cleanup(func() { resetRetryDelay(origInitial) })
	overrideRetryDelay(500 * time.Millisecond)

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Export(ctx, "tok", nil, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"expected context error, got: %v", err)
}

func TestExport_MalformedJSON(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))

	_, err := client.Export(context.Background(), "tok", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}

// --- ExportAll (pagination) ---

func TestExportAll_SinglePage(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExportResponse{
			Count:          1,
			NextPageCursor: nil,
			Results:        []BookData{{Title: "Only Book"}},
		})
	}))

	books, err := client.ExportAll(context.Background(), "tok", nil)
	require.NoError(t, err)
	require.Len(t, books, 1)
	assert.Equal(t, "Only Book", books[0].Title)
}

func TestExportAll_MultiplePages(t *testing.T) {
	cursor := "page2"
	var calls atomic.Int32

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("pageCursor") == "" {
			json.NewEncoder(w).Encode(ExportResponse{
				Count:          2,
				NextPageCursor: &cursor,
				Results:        []BookData{{Title: "Book 1"}},
			})
		} else {
			json.NewEncoder(w).Encode(ExportResponse{
				Count:          2,
				NextPageCursor: nil,
				Results:        []BookData{{Title: "Book 2"}},
			})
		}
	}))

	books, err := client.ExportAll(context.Background(), "tok", nil)
	require.NoError(t, err)
	require.Len(t, books, 2)
	assert.Equal(t, "Book 1", books[0].Title)
	assert.Equal(t, "Book 2", books[1].Title)
	assert.Equal(t, int32(2), calls.Load())
}

func TestExportAll_ErrorOnSecondPage(t *testing.T) {
	cursor := "page2"
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageCursor") == "" {
			json.NewEncoder(w).Encode(ExportResponse{
				NextPageCursor: &cursor,
				Results:        []BookData{{Title: "Book 1"}},
			})
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))

	_, err := client.ExportAll(context.Background(), "tok", nil)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestExportAll_EmptyCursorStopsPagination(t *testing.T) {
	emptyCursor := ""
	var calls atomic.Int32

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ExportResponse{
			NextPageCursor: &emptyCursor,
			Results:        []BookData{{Title: "Only"}},
		})
	}))

	books, err := client.ExportAll(context.Background(), "tok", nil)
	require.NoError(t, err)
	require.Len(t, books, 1)
	assert.Equal(t, int32(1), calls.Load(), "empty cursor should stop pagination")
}

// --- Helper functions ---

func TestCalculateRetryDelay(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{10, maxRetryDelay}, // capped
	}

	for _, tt := range tests {
		got := calculateRetryDelay(tt.attempt)
		assert.Equal(t, tt.want, got, "attempt %d", tt.attempt)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"rate limited", ErrRateLimited, true},
		{"server error 500", &ServerError{StatusCode: 500}, true},
		{"server error 503", &ServerError{StatusCode: 503}, true},
		{"invalid token", ErrInvalidToken, false},
		{"generic error", errors.New("network problem"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRetryableError(tt.err))
		})
	}
}

// --- NewClient / Options ---

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient()
	assert.Equal(t, defaultExportURL, c.exportURL)
	assert.Equal(t, defaultAuthURL, c.authURL)
	assert.NotNil(t, c.httpClient)
}

func TestNewClientWithOptions_BaseURL(t *testing.T) {
	c := NewClientWithOptions(WithBaseURL("http://localhost:9999"))
	assert.Equal(t, "http://localhost:9999/export/", c.exportURL)
	assert.Equal(t, "http://localhost:9999/auth/", c.authURL)
}

func TestNewClientWithOptions_HTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 5 * time.Second}
	c := NewClientWithOptions(WithHTTPClient(custom))
	assert.Same(t, custom, c.httpClient)
}

// --- doExportRequest edge cases ---

func TestDoExportRequest_ServerError(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))

	_, err := client.doExportRequest(context.Background(), client.exportURL, "tok")
	require.Error(t, err)

	var serverErr *ServerError
	require.True(t, errors.As(err, &serverErr))
	assert.Equal(t, http.StatusBadGateway, serverErr.StatusCode)
}

func TestDoExportRequest_UnexpectedStatus(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("nope"))
	}))

	_, err := client.doExportRequest(context.Background(), client.exportURL, "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 403")
	assert.Contains(t, err.Error(), "nope")
}
