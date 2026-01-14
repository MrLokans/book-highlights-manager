package dictionary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
)

// FreeDictionaryClient implements Client using the Free Dictionary API.
// API docs: https://dictionaryapi.dev/
type FreeDictionaryClient struct {
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

// NewFreeDictionaryClient creates a new Free Dictionary API client.
func NewFreeDictionaryClient() *FreeDictionaryClient {
	return &FreeDictionaryClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:     "https://api.dictionaryapi.dev/api/v2/entries/en",
		rateLimiter: newRateLimiter(500 * time.Millisecond),
	}
}

func (c *FreeDictionaryClient) Name() string {
	return "freedictionary"
}

// Lookup fetches word definitions from the Free Dictionary API.
func (c *FreeDictionaryClient) Lookup(ctx context.Context, word string) (*LookupResult, error) {
	word = strings.TrimSpace(strings.ToLower(word))
	if word == "" {
		return nil, fmt.Errorf("empty word")
	}

	c.rateLimiter.wait()

	url := fmt.Sprintf("%s/%s", c.baseURL, word)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "HighlightsManager/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch definition: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("word not found: %s", word)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var apiResponse []freeDictionaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResponse) == 0 {
		return nil, fmt.Errorf("empty response for word: %s", word)
	}

	return c.convertToLookupResult(word, apiResponse[0]), nil
}

func (c *FreeDictionaryClient) convertToLookupResult(word string, resp freeDictionaryResponse) *LookupResult {
	result := &LookupResult{
		Word: word,
	}

	// Extract pronunciation and audio from phonetics
	for _, phonetic := range resp.Phonetics {
		if result.Pronunciation == "" && phonetic.Text != "" {
			result.Pronunciation = phonetic.Text
		}
		if result.AudioURL == "" && phonetic.Audio != "" {
			result.AudioURL = phonetic.Audio
		}
	}

	// Extract definitions from meanings
	for _, meaning := range resp.Meanings {
		for _, def := range meaning.Definitions {
			wordDef := entities.WordDefinition{
				PartOfSpeech:  meaning.PartOfSpeech,
				Definition:    def.Definition,
				Example:       def.Example,
				Pronunciation: result.Pronunciation,
				AudioURL:      result.AudioURL,
				Source:        "freedictionary",
			}
			result.Definitions = append(result.Definitions, wordDef)
		}
	}

	return result
}

// Free Dictionary API response types

type freeDictionaryResponse struct {
	Word      string             `json:"word"`
	Phonetics []freeDictPhonetic `json:"phonetics"`
	Meanings  []freeDictMeaning  `json:"meanings"`
}

type freeDictPhonetic struct {
	Text  string `json:"text"`
	Audio string `json:"audio"`
}

type freeDictMeaning struct {
	PartOfSpeech string               `json:"partOfSpeech"`
	Definitions  []freeDictDefinition `json:"definitions"`
}

type freeDictDefinition struct {
	Definition string `json:"definition"`
	Example    string `json:"example"`
}
