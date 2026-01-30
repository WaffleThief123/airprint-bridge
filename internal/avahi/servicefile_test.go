package avahi

import (
	"strings"
	"testing"
)

func TestGenerateServiceFile(t *testing.T) {
	txtRecords := map[string]string{
		"txtvers": "1",
		"rp":      "printers/TestPrinter",
		"Color":   "T",
		"Duplex":  "F",
	}

	content, err := GenerateServiceFile("TestPrinter", 631, txtRecords)
	if err != nil {
		t.Fatalf("GenerateServiceFile() error = %v", err)
	}

	xml := string(content)

	// Check XML header
	if !strings.HasPrefix(xml, "<?xml version=") {
		t.Error("missing XML declaration")
	}

	// Check DOCTYPE
	if !strings.Contains(xml, "<!DOCTYPE service-group") {
		t.Error("missing DOCTYPE")
	}

	// Check service-group structure
	if !strings.Contains(xml, "<service-group>") {
		t.Error("missing service-group element")
	}

	// Check name with hostname placeholder
	if !strings.Contains(xml, "TestPrinter @ %h") {
		t.Error("missing printer name with hostname placeholder")
	}

	// Check service type
	if !strings.Contains(xml, "<type>_ipp._tcp</type>") {
		t.Error("missing IPP service type")
	}

	// Check subtype
	if !strings.Contains(xml, "_universal._sub._ipp._tcp") {
		t.Error("missing universal subtype")
	}

	// Check port
	if !strings.Contains(xml, "<port>631</port>") {
		t.Error("missing port element")
	}

	// Check TXT records are present
	if !strings.Contains(xml, "txtvers=1") {
		t.Error("missing txtvers record")
	}
	if !strings.Contains(xml, "Color=T") {
		t.Error("missing Color record")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Simple_Printer", "Simple Printer"},
		{"Printer<script>", "Printer script"},
		{"Normal Name", "Normal Name"},
		{"Has&Ampersand", "Has Ampersand"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestServiceFileName(t *testing.T) {
	tests := []struct {
		prefix      string
		printerName string
		want        string
	}{
		{"airprint-", "MyPrinter", "airprint-MyPrinter.service"},
		{"airprint-", "Printer With Spaces", "airprint-Printer_With_Spaces.service"},
		{"", "Printer", "Printer.service"},
		{"prefix-", "Name/With/Slashes", "prefix-Name_With_Slashes.service"},
	}

	for _, tt := range tests {
		t.Run(tt.printerName, func(t *testing.T) {
			got := ServiceFileName(tt.prefix, tt.printerName)
			if got != tt.want {
				t.Errorf("ServiceFileName(%q, %q) = %q, want %q", tt.prefix, tt.printerName, got, tt.want)
			}
		})
	}
}
