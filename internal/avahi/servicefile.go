package avahi

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

// ServiceGroup represents an Avahi service group XML structure
type ServiceGroup struct {
	XMLName xml.Name  `xml:"service-group"`
	Name    string    `xml:"name"`
	Service []Service `xml:"service"`
}

// Service represents a single service within a service group
type Service struct {
	Type      string      `xml:"type"`
	SubTypes  []string    `xml:"subtype,omitempty"`
	Port      int         `xml:"port"`
	TXTRecord []TXTRecord `xml:"txt-record"`
}

// TXTRecord represents a DNS-SD TXT record
type TXTRecord struct {
	Value string `xml:",chardata"`
}

// GenerateServiceFile creates an Avahi service file XML for a printer
func GenerateServiceFile(printerName string, port int, txtRecords map[string]string) ([]byte, error) {
	// Create sorted TXT records for consistent output
	var records []TXTRecord
	keys := make([]string, 0, len(txtRecords))
	for k := range txtRecords {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		records = append(records, TXTRecord{
			Value: fmt.Sprintf("%s=%s", k, txtRecords[k]),
		})
	}

	sg := ServiceGroup{
		Name: fmt.Sprintf("%s @ %%h", sanitizeName(printerName)),
		Service: []Service{
			{
				Type: "_ipp._tcp",
				SubTypes: []string{
					"_universal._sub._ipp._tcp",
				},
				Port:      port,
				TXTRecord: records,
			},
		},
	}

	// Generate XML with proper header and DOCTYPE
	output, err := xml.MarshalIndent(sg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service file: %w", err)
	}

	// Prepend XML declaration and DOCTYPE
	header := `<?xml version="1.0" standalone="no"?>
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
`
	return []byte(header + string(output) + "\n"), nil
}

// sanitizeName cleans a printer name for use in Avahi service names
func sanitizeName(name string) string {
	// Replace underscores with spaces for readability
	name = strings.ReplaceAll(name, "_", " ")

	// Remove or replace problematic characters
	var builder strings.Builder
	for _, r := range name {
		if r == '<' || r == '>' || r == '&' || r == '"' || r == '\'' {
			builder.WriteRune(' ')
		} else {
			builder.WriteRune(r)
		}
	}

	return strings.TrimSpace(builder.String())
}

// ServiceFileName returns the expected filename for a printer's service file
func ServiceFileName(prefix, printerName string) string {
	// Sanitize printer name for filesystem
	safeName := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, printerName)

	return fmt.Sprintf("%s%s.service", prefix, safeName)
}
