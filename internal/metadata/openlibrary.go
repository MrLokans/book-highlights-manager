package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// BookMetadata contains enriched book information from external sources.
type BookMetadata struct {
	Title           string   `json:"title,omitempty"`
	Author          string   `json:"author,omitempty"`
	ISBN            string   `json:"isbn,omitempty"`
	CoverURL        string   `json:"cover_url,omitempty"`
	Publisher       string   `json:"publisher,omitempty"`
	PublicationYear int      `json:"publication_year,omitempty"`
	Description     string   `json:"description,omitempty"`
	Subjects        []string `json:"subjects,omitempty"`
	PageCount       int      `json:"page_count,omitempty"`
	OpenLibraryKey  string   `json:"open_library_key,omitempty"`
}

// OpenLibraryClient fetches book metadata from the OpenLibrary API.
type OpenLibraryClient struct {
	httpClient  *http.Client
	baseURL     string
	rateLimiter *rateLimiter
}

type rateLimiter struct {
	mu       sync.Mutex
	lastCall time.Time
	interval time.Duration
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{interval: interval}
}

func (r *rateLimiter) wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	since := time.Since(r.lastCall)
	if since < r.interval {
		time.Sleep(r.interval - since)
	}
	r.lastCall = time.Now()
}

// NewOpenLibraryClient creates a new OpenLibrary API client with rate limiting.
func NewOpenLibraryClient() *OpenLibraryClient {
	return &OpenLibraryClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:     "https://openlibrary.org",
		rateLimiter: newRateLimiter(time.Second), // 1 request per second
	}
}

// SearchByISBN looks up a book by its ISBN and returns metadata.
func (c *OpenLibraryClient) SearchByISBN(ctx context.Context, isbn string) (*BookMetadata, error) {
	isbn = normalizeISBN(isbn)
	if isbn == "" {
		return nil, fmt.Errorf("invalid ISBN")
	}

	c.rateLimiter.wait()

	url := fmt.Sprintf("%s/isbn/%s.json", c.baseURL, isbn)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "HighlightsManager/1.0 (https://github.com/mrlokans/assistant)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch ISBN data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("ISBN not found: %s", isbn)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var bookData openLibraryBook
	if err := json.NewDecoder(resp.Body).Decode(&bookData); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	metadata := c.convertToMetadata(&bookData, isbn)

	// Fetch additional author info if we have author references
	if len(bookData.Authors) > 0 && metadata.Author == "" {
		authorName, err := c.fetchAuthorName(ctx, bookData.Authors[0].Key)
		if err == nil {
			metadata.Author = authorName
		}
	}

	return metadata, nil
}

// SearchByTitle looks up a book by title and author, returning the best match.
func (c *OpenLibraryClient) SearchByTitle(ctx context.Context, title, author string) (*BookMetadata, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	c.rateLimiter.wait()

	// Build search query
	q := url.QueryEscape(title)
	if author != "" {
		q = url.QueryEscape(fmt.Sprintf("%s %s", title, author))
	}

	searchURL := fmt.Sprintf("%s/search.json?q=%s&limit=5", c.baseURL, q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "HighlightsManager/1.0 (https://github.com/mrlokans/assistant)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search books: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var searchResult openLibrarySearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	if len(searchResult.Docs) == 0 {
		return nil, fmt.Errorf("no results found for: %s", title)
	}

	// Find the best match - prefer exact title match and matching author
	bestDoc := c.findBestMatch(searchResult.Docs, title, author)

	metadata := c.convertSearchDocToMetadata(bestDoc)

	// If no ISBN found but we have a cover edition key, fetch edition details
	if metadata.ISBN == "" && bestDoc.CoverEditionKey != "" {
		edition, err := c.fetchEditionDetails(ctx, bestDoc.CoverEditionKey)
		if err == nil {
			c.enrichMetadataFromEdition(metadata, edition)
		}
	}

	return metadata, nil
}

func (c *OpenLibraryClient) findBestMatch(docs []openLibrarySearchDoc, title, author string) *openLibrarySearchDoc {
	titleLower := strings.ToLower(title)
	authorLower := strings.ToLower(author)

	var bestMatch *openLibrarySearchDoc
	bestScore := -1

	for i := range docs {
		doc := &docs[i]
		score := 0

		// Exact title match
		if strings.ToLower(doc.Title) == titleLower {
			score += 10
		} else if strings.Contains(strings.ToLower(doc.Title), titleLower) {
			score += 5
		}

		// Author match
		if author != "" && len(doc.AuthorName) > 0 {
			for _, docAuthor := range doc.AuthorName {
				if strings.ToLower(docAuthor) == authorLower {
					score += 10
					break
				} else if strings.Contains(strings.ToLower(docAuthor), authorLower) {
					score += 5
					break
				}
			}
		}

		// Prefer books with ISBNs
		if len(doc.ISBN) > 0 {
			score += 2
		}

		// Prefer books with covers
		if doc.CoverI != 0 {
			score += 1
		}

		if score > bestScore {
			bestScore = score
			bestMatch = doc
		}
	}

	if bestMatch == nil && len(docs) > 0 {
		bestMatch = &docs[0]
	}

	return bestMatch
}

// fetchEditionDetails fetches detailed edition info including ISBN.
func (c *OpenLibraryClient) fetchEditionDetails(ctx context.Context, editionKey string) (*openLibraryEdition, error) {
	if editionKey == "" {
		return nil, fmt.Errorf("empty edition key")
	}

	c.rateLimiter.wait()

	url := fmt.Sprintf("%s/books/%s.json", c.baseURL, editionKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "HighlightsManager/1.0 (https://github.com/mrlokans/assistant)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var edition openLibraryEdition
	if err := json.NewDecoder(resp.Body).Decode(&edition); err != nil {
		return nil, err
	}

	return &edition, nil
}

func (c *OpenLibraryClient) fetchAuthorName(ctx context.Context, authorKey string) (string, error) {
	if authorKey == "" {
		return "", fmt.Errorf("empty author key")
	}

	c.rateLimiter.wait()

	url := fmt.Sprintf("%s%s.json", c.baseURL, authorKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "HighlightsManager/1.0 (https://github.com/mrlokans/assistant)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status: %d", resp.StatusCode)
	}

	var authorData struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authorData); err != nil {
		return "", err
	}

	return authorData.Name, nil
}

func (c *OpenLibraryClient) convertToMetadata(book *openLibraryBook, isbn string) *BookMetadata {
	metadata := &BookMetadata{
		Title:          book.Title,
		ISBN:           isbn,
		OpenLibraryKey: book.Key,
		PageCount:      book.NumberOfPages,
	}

	// Build cover URL using ISBN
	if isbn != "" {
		metadata.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/isbn/%s-L.jpg", isbn)
	}

	// Extract publication year
	if book.PublishDate != "" {
		metadata.PublicationYear = extractYear(book.PublishDate)
	}

	// Extract publisher (first one)
	if len(book.Publishers) > 0 {
		metadata.Publisher = book.Publishers[0]
	}

	// Extract description
	switch v := book.Description.(type) {
	case string:
		metadata.Description = v
	case map[string]any:
		if val, ok := v["value"].(string); ok {
			metadata.Description = val
		}
	}

	// Extract subjects
	if len(book.Subjects) > 0 {
		metadata.Subjects = book.Subjects
	}

	return metadata
}

func (c *OpenLibraryClient) enrichMetadataFromEdition(metadata *BookMetadata, edition *openLibraryEdition) {
	// Extract ISBN (prefer ISBN-13)
	if metadata.ISBN == "" {
		if len(edition.ISBN13) > 0 {
			metadata.ISBN = edition.ISBN13[0]
		} else if len(edition.ISBN10) > 0 {
			metadata.ISBN = edition.ISBN10[0]
		}
	}

	// Update cover URL if we now have ISBN but no cover
	if metadata.ISBN != "" && metadata.CoverURL == "" {
		metadata.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/isbn/%s-L.jpg", metadata.ISBN)
	}

	// Fill in publisher if missing
	if metadata.Publisher == "" && len(edition.Publishers) > 0 {
		metadata.Publisher = edition.Publishers[0]
	}

	// Fill in page count if missing
	if metadata.PageCount == 0 && edition.NumberOfPages > 0 {
		metadata.PageCount = edition.NumberOfPages
	}

	// Extract publication year if missing
	if metadata.PublicationYear == 0 && edition.PublishDate != "" {
		metadata.PublicationYear = extractYear(edition.PublishDate)
	}
}

func (c *OpenLibraryClient) convertSearchDocToMetadata(doc *openLibrarySearchDoc) *BookMetadata {
	metadata := &BookMetadata{
		Title:           doc.Title,
		PublicationYear: doc.FirstPublishYear,
	}

	if len(doc.AuthorName) > 0 {
		metadata.Author = doc.AuthorName[0]
	}

	if len(doc.Publisher) > 0 {
		metadata.Publisher = doc.Publisher[0]
	}

	if len(doc.ISBN) > 0 {
		metadata.ISBN = doc.ISBN[0]
		metadata.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/isbn/%s-L.jpg", doc.ISBN[0])
	} else if doc.CoverI != 0 {
		metadata.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", doc.CoverI)
	}

	if len(doc.Subject) > 0 {
		metadata.Subjects = doc.Subject
		if len(metadata.Subjects) > 10 {
			metadata.Subjects = metadata.Subjects[:10]
		}
	}

	if doc.Key != "" {
		metadata.OpenLibraryKey = doc.Key
	}

	return metadata
}

// normalizeISBN removes hyphens and spaces from ISBN.
func normalizeISBN(isbn string) string {
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	isbn = strings.TrimSpace(isbn)

	// Basic validation: ISBN-10 or ISBN-13
	if len(isbn) != 10 && len(isbn) != 13 {
		return ""
	}

	return isbn
}

// extractYear tries to extract a 4-digit year from a date string.
func extractYear(dateStr string) int {
	dateStr = strings.TrimSpace(dateStr)
	if len(dateStr) < 4 {
		return 0
	}

	// Try parsing common formats
	formats := []string{
		"2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2006-01-02",
		"January 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Year()
		}
	}

	// Last resort: find 4 consecutive digits
	for i := 0; i <= len(dateStr)-4; i++ {
		if dateStr[i] >= '0' && dateStr[i] <= '9' {
			yearStr := dateStr[i : i+4]
			var year int
			if _, err := fmt.Sscanf(yearStr, "%d", &year); err == nil && year > 1000 && year < 3000 {
				return year
			}
		}
	}

	return 0
}

// OpenLibrary API response types (internal)

type openLibraryBook struct {
	Key           string      `json:"key"`
	Title         string      `json:"title"`
	Authors       []authorRef `json:"authors"`
	Publishers    []string    `json:"publishers"`
	PublishDate   string      `json:"publish_date"`
	NumberOfPages int         `json:"number_of_pages"`
	Description   any         `json:"description"` // Can be string or {type, value}
	Subjects      []string    `json:"subjects"`
	Covers        []int       `json:"covers"`
}

type authorRef struct {
	Key string `json:"key"`
}

type openLibrarySearchResult struct {
	NumFound int                    `json:"numFound"`
	Docs     []openLibrarySearchDoc `json:"docs"`
}

type openLibrarySearchDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	Publisher        []string `json:"publisher"`
	ISBN             []string `json:"isbn"`
	CoverI           int      `json:"cover_i"`
	CoverEditionKey  string   `json:"cover_edition_key"`
	Subject          []string `json:"subject"`
}

type openLibraryEdition struct {
	Key           string   `json:"key"`
	Title         string   `json:"title"`
	Publishers    []string `json:"publishers"`
	PublishDate   string   `json:"publish_date"`
	ISBN10        []string `json:"isbn_10"`
	ISBN13        []string `json:"isbn_13"`
	NumberOfPages int      `json:"number_of_pages"`
	Covers        []int    `json:"covers"`
}
