package dictionary

import (
	"context"

	"github.com/mrlokans/assistant/internal/entities"
)

// LookupResult contains the result of a dictionary lookup.
type LookupResult struct {
	Word          string
	Definitions   []entities.WordDefinition
	Pronunciation string
	AudioURL      string
}

// Client defines the interface for dictionary API providers.
type Client interface {
	Lookup(ctx context.Context, word string) (*LookupResult, error)
	Name() string
}
