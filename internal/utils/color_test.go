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
		{"converts negative to hex", "-256", "#FFFFFF00", false},
		{"converts positive to hex", "16777216", "#01000000", false},
		{"converts zero", "0", "#00000000", false},
		{"returns error for invalid", "not-a-number", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := InternalColorToHexARGB(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestColorToCalloutType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"#FFFFFF00", "quote"},
		{"#FF00FF00", "note"},
		{"#FFFF0000", "warning"},
		{"#FF0000FF", "info"},
		{"#FFFF00FF", "tip"},
		{"#FF123456", "quote"},
		{"unknown", "quote"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ColorToCalloutType(tt.input))
		})
	}
}
