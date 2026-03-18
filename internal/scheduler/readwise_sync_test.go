package scheduler

import (
	"testing"
	"time"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/readwise"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertReadwiseBook(t *testing.T) {
	t.Run("converts basic fields", func(t *testing.T) {
		data := readwise.BookData{
			Title:         "Test Book",
			Author:        "Test Author",
			CoverImageURL: "https://example.com/cover.jpg",
			ASIN:          "B001234567",
			UserBookID:    42,
		}

		book := convertReadwiseBook(data, 5)

		assert.Equal(t, "Test Book", book.Title)
		assert.Equal(t, "Test Author", book.Author)
		assert.Equal(t, "https://example.com/cover.jpg", book.CoverURL)
		assert.Equal(t, "B001234567", book.ASIN)
		assert.Equal(t, "42", book.ExternalID)
		assert.Equal(t, uint(5), book.SourceID)
	})

	t.Run("converts highlights", func(t *testing.T) {
		now := time.Now()
		data := readwise.BookData{
			Title:  "Book",
			Author: "Author",
			Highlights: []readwise.HighlightData{
				{ID: 1, Text: "highlight 1", HighlightedAt: now},
				{ID: 2, Text: "highlight 2", Note: "my note", IsFavorite: true},
			},
		}

		book := convertReadwiseBook(data, 1)

		require.Len(t, book.Highlights, 2)
		assert.Equal(t, "highlight 1", book.Highlights[0].Text)
		assert.Equal(t, "1", book.Highlights[0].ExternalID)
		assert.Equal(t, uint(1), book.Highlights[0].SourceID)
		assert.Equal(t, "my note", book.Highlights[1].Note)
		assert.True(t, book.Highlights[1].IsFavorite)
	})
}

func TestConvertReadwiseHighlight(t *testing.T) {
	now := time.Now()

	t.Run("maps all fields", func(t *testing.T) {
		data := readwise.HighlightData{
			ID:            123,
			Text:          "some text",
			Note:          "my note",
			Location:      42,
			EndLocation:   50,
			Color:         "yellow",
			HighlightedAt: now,
			IsFavorite:    true,
			IsDiscarded:   false,
			LocationType:  "page",
		}

		h := convertReadwiseHighlight(data, 7)

		assert.Equal(t, "some text", h.Text)
		assert.Equal(t, "my note", h.Note)
		assert.Equal(t, 42, h.LocationValue)
		assert.Equal(t, 50, h.LocationEnd)
		assert.Equal(t, "yellow", h.Color)
		assert.Equal(t, now, h.HighlightedAt)
		assert.True(t, h.IsFavorite)
		assert.False(t, h.IsDiscarded)
		assert.Equal(t, "123", h.ExternalID)
		assert.Equal(t, uint(7), h.SourceID)
		assert.Equal(t, entities.LocationTypePage, h.LocationType)
	})
}

func TestMapLocationTypeFromReadwise(t *testing.T) {
	tests := []struct {
		input    string
		expected entities.LocationType
	}{
		{"page", entities.LocationTypePage},
		{"location", entities.LocationTypeLocation},
		{"time_offset", entities.LocationTypeTime},
		{"order", entities.LocationTypePosition},
		{"unknown", entities.LocationTypeNone},
		{"", entities.LocationTypeNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapLocationTypeFromReadwise(tt.input))
		})
	}
}

func TestReadwiseSyncScheduler_StateManagement(t *testing.T) {
	t.Run("initially not running", func(t *testing.T) {
		sched := NewReadwiseSyncScheduler(nil, nil, nil, nil, nil)
		assert.False(t, sched.IsRunning())
		assert.False(t, sched.IsSyncing())
		assert.Nil(t, sched.GetNextRunTime())
	})
}
