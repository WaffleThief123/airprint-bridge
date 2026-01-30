package airprint

import (
	"fmt"
	"strings"

	"github.com/cyra/airprint-cups-plugin/internal/cups"
)

// TXTRecords holds the DNS-SD TXT records for AirPrint advertisement
type TXTRecords struct {
	records map[string]string
}

// NewTXTRecords creates TXT records from a CUPS printer
func NewTXTRecords(printer *cups.Printer) *TXTRecords {
	t := &TXTRecords{
		records: make(map[string]string),
	}

	// Required AirPrint records
	t.Set("txtvers", "1")
	t.Set("qtotal", "1")

	// Resource path for the printer
	t.Set("rp", fmt.Sprintf("printers/%s", printer.Name))

	// Printer description
	if printer.MakeModel != "" {
		t.Set("ty", printer.MakeModel)
	} else {
		t.Set("ty", printer.Name)
	}

	// Location note
	if printer.Location != "" {
		t.Set("note", printer.Location)
	} else if printer.Info != "" {
		t.Set("note", printer.Info)
	}

	// Supported document formats (PDLs)
	// Order matters: URF should be first for AirPrint
	t.Set("pdl", "image/urf,application/pdf,image/jpeg,image/png")

	// URF capabilities string
	urf := NewURFCapabilities(
		printer.ColorSupported,
		printer.DuplexSupported,
		printer.Resolutions,
	)
	t.Set("URF", urf.String())

	// Color support
	if printer.ColorSupported {
		t.Set("Color", "T")
	} else {
		t.Set("Color", "F")
	}

	// Duplex support
	if printer.DuplexSupported {
		t.Set("Duplex", "T")
	} else {
		t.Set("Duplex", "F")
	}

	// NOTE: We deliberately skip the "media" TXT record because:
	// 1. It can be very long for label printers and break Avahi
	// 2. iOS queries the printer directly via IPP for media sizes
	// 3. It's optional for AirPrint discovery

	// Additional AirPrint identifiers
	t.Set("product", fmt.Sprintf("(%s)", sanitizeProduct(printer.MakeModel)))
	t.Set("priority", "50") // Middle priority

	// Transparent printing support
	t.Set("Transparent", "F")

	// Binary protocol support
	t.Set("Binary", "F")

	// TBCP (Tagged Binary Communication Protocol)
	t.Set("TBCP", "F")

	return t
}

// Set adds or updates a TXT record
func (t *TXTRecords) Set(key, value string) {
	t.records[key] = value
}

// Get retrieves a TXT record value
func (t *TXTRecords) Get(key string) (string, bool) {
	v, ok := t.records[key]
	return v, ok
}

// All returns all records as a map
func (t *TXTRecords) All() map[string]string {
	result := make(map[string]string, len(t.records))
	for k, v := range t.records {
		result[k] = v
	}
	return result
}

// Pairs returns all records as key=value pairs
func (t *TXTRecords) Pairs() []string {
	pairs := make([]string, 0, len(t.records))
	for k, v := range t.records {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return pairs
}

// sanitizeProduct cleans the product name for use in TXT records
func sanitizeProduct(model string) string {
	if model == "" {
		return "Unknown Printer"
	}

	// Remove problematic characters
	model = strings.ReplaceAll(model, "(", "")
	model = strings.ReplaceAll(model, ")", "")

	// Truncate if too long
	if len(model) > 128 {
		model = model[:128]
	}

	return model
}
