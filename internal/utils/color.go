package utils

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

// InternalColorToHexARGB converts MoonReader's signed integer color representation
// to ARGB hex format.
// Example: "-15654349" -> "#FF112233"
func InternalColorToHexARGB(colorStr string) (string, error) {
	colorInt, err := strconv.ParseInt(colorStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse color string: %w", err)
	}

	// Convert to unsigned 32-bit representation (2's complement)
	colorUint := uint32(colorInt)

	// Convert to hex bytes (big-endian)
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, colorUint)

	return fmt.Sprintf("#%02X%02X%02X%02X", bytes[0], bytes[1], bytes[2], bytes[3]), nil
}

// ColorToCalloutType maps hex ARGB colors to Obsidian callout types.
// Default return is "quote" for unknown colors.
func ColorToCalloutType(hexColor string) string {
	colorMapping := map[string]string{
		"#FFFFFF00": "quote",   // Yellow highlights -> quotes
		"#FF00FF00": "note",    // Green highlights -> notes
		"#FFFF0000": "warning", // Red highlights -> warnings
		"#FF0000FF": "info",    // Blue highlights -> info
		"#FFFF00FF": "tip",     // Magenta highlights -> tips
	}

	if calloutType, ok := colorMapping[hexColor]; ok {
		return calloutType
	}
	return "quote"
}
