package cups

import (
	"reflect"
	"testing"
)

func TestParseResolutions(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   []int
	}{
		{
			name:   "single resolution",
			values: []string{"300dpi"},
			want:   []int{300},
		},
		{
			name:   "square resolution",
			values: []string{"600x600dpi"},
			want:   []int{600},
		},
		{
			name:   "asymmetric resolution",
			values: []string{"300x600dpi"},
			want:   []int{300, 600},
		},
		{
			name:   "multiple resolutions",
			values: []string{"300dpi", "600dpi", "1200dpi"},
			want:   []int{300, 600, 1200},
		},
		{
			name:   "empty",
			values: []string{},
			want:   nil,
		},
		{
			name:   "invalid format",
			values: []string{"not a resolution"},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseResolutions(tt.values)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseResolutions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDuplexSupport(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   bool
	}{
		{
			name:   "one-sided only",
			values: []string{"one-sided"},
			want:   false,
		},
		{
			name:   "two-sided long edge",
			values: []string{"one-sided", "two-sided-long-edge"},
			want:   true,
		},
		{
			name:   "two-sided short edge",
			values: []string{"one-sided", "two-sided-short-edge"},
			want:   true,
		},
		{
			name:   "duplex keyword",
			values: []string{"simplex", "duplex"},
			want:   true,
		},
		{
			name:   "empty",
			values: []string{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDuplexSupport(tt.values)
			if got != tt.want {
				t.Errorf("ParseDuplexSupport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizePaperSize(t *testing.T) {
	tests := []struct {
		media string
		want  string
	}{
		{"iso_a4_210x297mm", "A4"},
		{"na_letter_8.5x11in", "Letter"},
		{"iso_a3_297x420mm", "A3"},
		{"na_legal_8.5x14in", "Legal"},
		{"custom_100x150mm", "custom_100x150mm"},
	}

	for _, tt := range tests {
		t.Run(tt.media, func(t *testing.T) {
			got := NormalizePaperSize(tt.media)
			if got != tt.want {
				t.Errorf("NormalizePaperSize(%q) = %q, want %q", tt.media, got, tt.want)
			}
		})
	}
}

func TestGetDefaultResolution(t *testing.T) {
	tests := []struct {
		name        string
		resolutions []int
		want        int
	}{
		{
			name:        "empty uses fallback",
			resolutions: []int{},
			want:        300,
		},
		{
			name:        "prefers 300",
			resolutions: []int{150, 300, 1200},
			want:        300,
		},
		{
			name:        "prefers 600",
			resolutions: []int{150, 600, 1200},
			want:        600,
		},
		{
			name:        "uses highest if no 300/600",
			resolutions: []int{150, 1200},
			want:        1200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDefaultResolution(tt.resolutions)
			if got != tt.want {
				t.Errorf("GetDefaultResolution() = %v, want %v", got, tt.want)
			}
		})
	}
}
