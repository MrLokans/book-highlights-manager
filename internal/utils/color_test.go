package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalColorToHexARGB(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "negative color value",
			input:    "-15654349",
			expected: "#FF112233",
			wantErr:  false,
		},
		{
			name:     "positive color value",
			input:    "1996532479",
			expected: "#7700AAFF",
			wantErr:  false,
		},
		{
			name:     "yellow color",
			input:    "-256",
			expected: "#FFFFFF00",
			wantErr:  false,
		},
		{
			name:     "pure white",
			input:    "-1",
			expected: "#FFFFFFFF",
			wantErr:  false,
		},
		{
			name:     "pure black",
			input:    "-16777216",
			expected: "#FF000000",
			wantErr:  false,
		},
		{
			name:    "invalid input",
			input:   "not-a-number",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := InternalColorToHexARGB(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorToCalloutType(t *testing.T) {
	tests := []struct {
		name     string
		hexColor string
		expected string
	}{
		{
			name:     "yellow -> quote",
			hexColor: "#FFFFFF00",
			expected: "quote",
		},
		{
			name:     "green -> note",
			hexColor: "#FF00FF00",
			expected: "note",
		},
		{
			name:     "red -> warning",
			hexColor: "#FFFF0000",
			expected: "warning",
		},
		{
			name:     "blue -> info",
			hexColor: "#FF0000FF",
			expected: "info",
		},
		{
			name:     "magenta -> tip",
			hexColor: "#FFFF00FF",
			expected: "tip",
		},
		{
			name:     "unknown color -> quote",
			hexColor: "#FF123456",
			expected: "quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorToCalloutType(tt.hexColor)
			assert.Equal(t, tt.expected, result)
		})
	}
}
