package airprint

import (
	"testing"

	"github.com/cyra/airprint-cups-plugin/internal/cups"
)

func TestNewTXTRecords(t *testing.T) {
	printer := &cups.Printer{
		Name:            "TestPrinter",
		MakeModel:       "Test Printer Model",
		Location:        "Office",
		ColorSupported:  true,
		DuplexSupported: true,
		Resolutions:     []int{300, 600},
		MediaSupported:  []string{"iso_a4_210x297mm", "na_letter_8.5x11in"},
	}

	records := NewTXTRecords(printer)

	// Check required records
	requiredRecords := map[string]string{
		"txtvers": "1",
		"rp":      "printers/TestPrinter",
		"ty":      "Test Printer Model",
		"note":    "Office",
		"Color":   "T",
		"Duplex":  "T",
	}

	for key, want := range requiredRecords {
		got, ok := records.Get(key)
		if !ok {
			t.Errorf("missing required record %q", key)
			continue
		}
		if got != want {
			t.Errorf("record %q = %q, want %q", key, got, want)
		}
	}

	// Check PDL includes URF
	pdl, ok := records.Get("pdl")
	if !ok {
		t.Error("missing pdl record")
	}
	if pdl == "" {
		t.Error("pdl record is empty")
	}

	// Check URF exists and is not empty
	urf, ok := records.Get("URF")
	if !ok {
		t.Error("missing URF record")
	}
	if urf == "" {
		t.Error("URF record is empty")
	}
}

func TestTXTRecords_ColorValues(t *testing.T) {
	tests := []struct {
		name           string
		colorSupported bool
		wantColor      string
	}{
		{"color printer", true, "T"},
		{"grayscale printer", false, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printer := &cups.Printer{
				Name:           "Test",
				ColorSupported: tt.colorSupported,
			}
			records := NewTXTRecords(printer)

			got, _ := records.Get("Color")
			if got != tt.wantColor {
				t.Errorf("Color = %q, want %q", got, tt.wantColor)
			}
		})
	}
}

func TestTXTRecords_DuplexValues(t *testing.T) {
	tests := []struct {
		name            string
		duplexSupported bool
		wantDuplex      string
	}{
		{"duplex printer", true, "T"},
		{"simplex printer", false, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printer := &cups.Printer{
				Name:            "Test",
				DuplexSupported: tt.duplexSupported,
			}
			records := NewTXTRecords(printer)

			got, _ := records.Get("Duplex")
			if got != tt.wantDuplex {
				t.Errorf("Duplex = %q, want %q", got, tt.wantDuplex)
			}
		})
	}
}

func TestTXTRecords_Pairs(t *testing.T) {
	printer := &cups.Printer{
		Name: "Test",
	}
	records := NewTXTRecords(printer)
	pairs := records.Pairs()

	if len(pairs) == 0 {
		t.Error("Pairs() returned empty slice")
	}

	// Check format is key=value
	for _, pair := range pairs {
		if len(pair) < 3 { // minimum: "k=v"
			t.Errorf("invalid pair format: %q", pair)
		}
	}
}
