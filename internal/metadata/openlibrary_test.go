package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeISBN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"978-0-13-468599-1", "9780134685991"},
		{"0-13-468599-6", "0134685996"},
		{"978 0 13 468599 1", "9780134685991"},
		{"9780134685991", "9780134685991"},
		{"0134685996", "0134685996"},
		{"123", ""},            // Too short
		{"12345678901234", ""}, // Too long
		{"", ""},
		{"  978-0-13-468599-1  ", "9780134685991"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeISBN(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeISBN(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"2020", 2020},
		{"January 15, 2019", 2019},
		{"Jan 15, 2019", 2019},
		{"2021-06-15", 2021},
		{"January 2018", 2018},
		{"Published in 1999", 1999},
		{"", 0},
		{"no year here", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractYear(tt.input)
			if result != tt.expected {
				t.Errorf("extractYear(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSearchByISBN(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/isbn/9780134685991.json" {
			response := openLibraryBook{
				Key:           "/books/OL123M",
				Title:         "Effective Java",
				Publishers:    []string{"Addison-Wesley"},
				PublishDate:   "2018",
				NumberOfPages: 416,
				Authors:       []authorRef{{Key: "/authors/OL456A"}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/authors/OL456A.json" {
			response := map[string]string{"name": "Joshua Bloch"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &OpenLibraryClient{
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		baseURL:     server.URL,
		rateLimiter: newRateLimiter(0), // No rate limiting for tests
	}

	ctx := context.Background()
	metadata, err := client.SearchByISBN(ctx, "978-0-13-468599-1")
	if err != nil {
		t.Fatalf("SearchByISBN failed: %v", err)
	}

	if metadata.Title != "Effective Java" {
		t.Errorf("expected title 'Effective Java', got %q", metadata.Title)
	}
	if metadata.Publisher != "Addison-Wesley" {
		t.Errorf("expected publisher 'Addison-Wesley', got %q", metadata.Publisher)
	}
	if metadata.PublicationYear != 2018 {
		t.Errorf("expected year 2018, got %d", metadata.PublicationYear)
	}
	if metadata.Author != "Joshua Bloch" {
		t.Errorf("expected author 'Joshua Bloch', got %q", metadata.Author)
	}
	if metadata.CoverURL == "" {
		t.Error("expected cover URL to be set")
	}
}

func TestSearchByISBN_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &OpenLibraryClient{
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		baseURL:     server.URL,
		rateLimiter: newRateLimiter(0),
	}

	ctx := context.Background()
	_, err := client.SearchByISBN(ctx, "0000000000")
	if err == nil {
		t.Error("expected error for non-existent ISBN")
	}
}

func TestSearchByISBN_InvalidISBN(t *testing.T) {
	client := NewOpenLibraryClient()
	ctx := context.Background()

	_, err := client.SearchByISBN(ctx, "invalid")
	if err == nil {
		t.Error("expected error for invalid ISBN")
	}
}

func TestSearchByTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search.json" {
			response := openLibrarySearchResult{
				NumFound: 1,
				Docs: []openLibrarySearchDoc{
					{
						Key:              "/works/OL789W",
						Title:            "Clean Code",
						AuthorName:       []string{"Robert C. Martin"},
						FirstPublishYear: 2008,
						Publisher:        []string{"Prentice Hall"},
						ISBN:             []string{"9780132350884"},
						CoverI:           12345,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &OpenLibraryClient{
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		baseURL:     server.URL,
		rateLimiter: newRateLimiter(0),
	}

	ctx := context.Background()
	metadata, err := client.SearchByTitle(ctx, "Clean Code", "Robert Martin")
	if err != nil {
		t.Fatalf("SearchByTitle failed: %v", err)
	}

	if metadata.Title != "Clean Code" {
		t.Errorf("expected title 'Clean Code', got %q", metadata.Title)
	}
	if metadata.Author != "Robert C. Martin" {
		t.Errorf("expected author 'Robert C. Martin', got %q", metadata.Author)
	}
	if metadata.ISBN != "9780132350884" {
		t.Errorf("expected ISBN '9780132350884', got %q", metadata.ISBN)
	}
}

func TestSearchByTitle_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := openLibrarySearchResult{NumFound: 0, Docs: []openLibrarySearchDoc{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &OpenLibraryClient{
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		baseURL:     server.URL,
		rateLimiter: newRateLimiter(0),
	}

	ctx := context.Background()
	_, err := client.SearchByTitle(ctx, "Nonexistent Book Title XYZ", "")
	if err == nil {
		t.Error("expected error for no results")
	}
}

func TestFindBestMatch(t *testing.T) {
	client := NewOpenLibraryClient()

	docs := []openLibrarySearchDoc{
		{Title: "Other Book", AuthorName: []string{"Someone Else"}},
		{Title: "Clean Code", AuthorName: []string{"Robert C. Martin"}, ISBN: []string{"123"}, CoverI: 1},
		{Title: "Clean Code: A Handbook", AuthorName: []string{"Another Author"}},
	}

	best := client.findBestMatch(docs, "Clean Code", "Robert Martin")

	if best.Title != "Clean Code" {
		t.Errorf("expected best match to be 'Clean Code', got %q", best.Title)
	}
	if len(best.AuthorName) == 0 || best.AuthorName[0] != "Robert C. Martin" {
		t.Errorf("expected best match author to be 'Robert C. Martin'")
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(50 * time.Millisecond)

	start := time.Now()
	rl.wait()
	rl.wait()
	elapsed := time.Since(start)

	// Second call should have waited at least 50ms
	if elapsed < 50*time.Millisecond {
		t.Errorf("rate limiter did not wait: elapsed=%v", elapsed)
	}
}
