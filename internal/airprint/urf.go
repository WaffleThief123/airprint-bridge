package airprint

import (
	"fmt"
	"sort"
	"strings"
)

// URFCapabilities represents URF (Unified Raster Format) capabilities
type URFCapabilities struct {
	ColorModes  []string // W8 (grayscale), SRGB24 (color)
	Duplex      []string // DM1 (simplex), DM3 (duplex-long), DM4 (duplex-short)
	Quality     []string // CP1-CP255 (print quality levels)
	Resolutions []int    // DPI values
}

// NewURFCapabilities creates URF capabilities from printer info
func NewURFCapabilities(colorSupported, duplexSupported bool, resolutions []int) *URFCapabilities {
	urf := &URFCapabilities{
		ColorModes:  []string{"W8"}, // Always support grayscale
		Duplex:      []string{"DM1"}, // Always support simplex
		Quality:     []string{"CP255"}, // Maximum quality
		Resolutions: resolutions,
	}

	if colorSupported {
		urf.ColorModes = append(urf.ColorModes, "SRGB24")
	}

	if duplexSupported {
		urf.Duplex = append(urf.Duplex, "DM3", "DM4")
	}

	// Ensure we have at least 300dpi
	if len(urf.Resolutions) == 0 {
		urf.Resolutions = []int{300}
	}

	return urf
}

// String returns the URF capability string for AirPrint TXT records
// Format: "W8,SRGB24,CP255,RS300-600,DM3"
func (u *URFCapabilities) String() string {
	var parts []string

	// Add color modes
	parts = append(parts, u.ColorModes...)

	// Add quality
	parts = append(parts, u.Quality...)

	// Add resolutions (format: RS<min>-<max> or RS<single>)
	parts = append(parts, u.resolutionString())

	// Add duplex modes
	parts = append(parts, u.Duplex...)

	return strings.Join(parts, ",")
}

// resolutionString returns the RS portion of the URF string
func (u *URFCapabilities) resolutionString() string {
	if len(u.Resolutions) == 0 {
		return "RS300"
	}

	// Sort and deduplicate
	sorted := make([]int, len(u.Resolutions))
	copy(sorted, u.Resolutions)
	sort.Ints(sorted)

	// Remove duplicates
	unique := []int{sorted[0]}
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1] {
			unique = append(unique, sorted[i])
		}
	}

	if len(unique) == 1 {
		return fmt.Sprintf("RS%d", unique[0])
	}

	// Return range
	return fmt.Sprintf("RS%d-%d", unique[0], unique[len(unique)-1])
}

// DefaultURFCapabilities returns sensible defaults when printer info is unavailable
func DefaultURFCapabilities() *URFCapabilities {
	return &URFCapabilities{
		ColorModes:  []string{"W8", "SRGB24"},
		Duplex:      []string{"DM1"},
		Quality:     []string{"CP255"},
		Resolutions: []int{300, 600},
	}
}
