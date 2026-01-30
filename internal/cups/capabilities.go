package cups

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseResolutions extracts DPI values from IPP resolution strings
// Formats: "300dpi", "600x600dpi", "300x600dpi"
func ParseResolutions(values []string) []int {
	seen := make(map[int]bool)
	var resolutions []int

	re := regexp.MustCompile(`(\d+)(?:x(\d+))?dpi`)

	for _, v := range values {
		matches := re.FindStringSubmatch(strings.ToLower(v))
		if len(matches) >= 2 {
			if dpi, err := strconv.Atoi(matches[1]); err == nil {
				if !seen[dpi] {
					seen[dpi] = true
					resolutions = append(resolutions, dpi)
				}
			}
			if len(matches) >= 3 && matches[2] != "" {
				if dpi, err := strconv.Atoi(matches[2]); err == nil {
					if !seen[dpi] {
						seen[dpi] = true
						resolutions = append(resolutions, dpi)
					}
				}
			}
		}
	}

	return resolutions
}

// ParseDuplexSupport checks if duplex is supported from sides-supported attribute
func ParseDuplexSupport(values []string) bool {
	for _, v := range values {
		v = strings.ToLower(v)
		if strings.Contains(v, "two-sided") || v == "duplex" {
			return true
		}
	}
	return false
}

// NormalizePaperSize converts CUPS media names to standard format
// Examples: "iso_a4_210x297mm" -> "A4", "na_letter_8.5x11in" -> "Letter"
func NormalizePaperSize(media string) string {
	media = strings.ToLower(media)

	paperMappings := map[string]string{
		"a4":        "A4",
		"a3":        "A3",
		"a5":        "A5",
		"letter":    "Letter",
		"legal":     "Legal",
		"tabloid":   "Tabloid",
		"executive": "Executive",
		"b5":        "B5",
		"4x6":       "4x6",
		"5x7":       "5x7",
	}

	for pattern, name := range paperMappings {
		if strings.Contains(media, pattern) {
			return name
		}
	}

	return media
}

// GetDefaultResolution returns a sensible default resolution from available options
func GetDefaultResolution(resolutions []int) int {
	if len(resolutions) == 0 {
		return 300 // Default fallback
	}

	// Prefer 300 or 600 DPI
	for _, dpi := range resolutions {
		if dpi == 300 || dpi == 600 {
			return dpi
		}
	}

	// Return highest available
	max := resolutions[0]
	for _, dpi := range resolutions[1:] {
		if dpi > max {
			max = dpi
		}
	}
	return max
}
