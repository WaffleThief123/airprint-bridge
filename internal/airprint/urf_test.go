package airprint

import (
	"strings"
	"testing"
)

func TestNewURFCapabilities(t *testing.T) {
	tests := []struct {
		name           string
		colorSupported bool
		duplexSupported bool
		resolutions    []int
		wantContains   []string
		wantNotContains []string
	}{
		{
			name:            "color and duplex",
			colorSupported:  true,
			duplexSupported: true,
			resolutions:     []int{300, 600},
			wantContains:    []string{"W8", "SRGB24", "DM1", "DM3", "DM4", "RS300-600"},
			wantNotContains: nil,
		},
		{
			name:            "grayscale only",
			colorSupported:  false,
			duplexSupported: false,
			resolutions:     []int{300},
			wantContains:    []string{"W8", "DM1", "RS300"},
			wantNotContains: []string{"SRGB24", "DM3", "DM4"},
		},
		{
			name:            "empty resolutions defaults to 300",
			colorSupported:  true,
			duplexSupported: false,
			resolutions:     nil,
			wantContains:    []string{"RS300"},
			wantNotContains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urf := NewURFCapabilities(tt.colorSupported, tt.duplexSupported, tt.resolutions)
			got := urf.String()

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("URF string %q should contain %q", got, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("URF string %q should not contain %q", got, notWant)
				}
			}
		})
	}
}

func TestURFCapabilities_resolutionString(t *testing.T) {
	tests := []struct {
		name        string
		resolutions []int
		want        string
	}{
		{
			name:        "single resolution",
			resolutions: []int{300},
			want:        "RS300",
		},
		{
			name:        "two resolutions",
			resolutions: []int{300, 600},
			want:        "RS300-600",
		},
		{
			name:        "unsorted resolutions",
			resolutions: []int{600, 300, 1200},
			want:        "RS300-1200",
		},
		{
			name:        "duplicate resolutions",
			resolutions: []int{300, 300, 600},
			want:        "RS300-600",
		},
		{
			name:        "empty defaults to 300",
			resolutions: nil,
			want:        "RS300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urf := &URFCapabilities{Resolutions: tt.resolutions}
			if len(tt.resolutions) == 0 {
				urf.Resolutions = []int{300}
			}
			got := urf.resolutionString()
			if got != tt.want {
				t.Errorf("resolutionString() = %q, want %q", got, tt.want)
			}
		})
	}
}
