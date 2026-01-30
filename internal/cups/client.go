package cups

import (
	"fmt"

	"github.com/phin1x/go-ipp"
)

// Client wraps the CUPS client for communication
type Client struct {
	cupsClient *ipp.CUPSClient
	host       string
	port       int
}

// Requested attributes for printer queries
var printerAttributes = []string{
	"printer-name",
	"printer-uri-supported",
	"printer-make-and-model",
	"printer-location",
	"printer-info",
	"printer-state",
	"printer-is-shared",
	"printer-is-accepting-jobs",
	"color-supported",
	"sides-supported",
	"printer-resolution-supported",
	"media-supported",
	"media-ready",
	"media-default",
}

// NewClient creates a new CUPS client
func NewClient(host string, port int) *Client {
	cupsClient := ipp.NewCUPSClient(
		host,
		port,
		"",    // username (empty for local)
		"",    // password
		false, // useTLS
	)

	return &Client{
		cupsClient: cupsClient,
		host:       host,
		port:       port,
	}
}

// GetPrinters returns all printers from CUPS
func (c *Client) GetPrinters() ([]Printer, error) {
	printerMap, err := c.cupsClient.GetPrinters(printerAttributes)
	if err != nil {
		return nil, fmt.Errorf("failed to get printers: %w", err)
	}

	var printers []Printer
	for name, attrs := range printerMap {
		printer := c.parsePrinterAttributes(name, attrs)
		printers = append(printers, printer)
	}

	return printers, nil
}

// GetPrinter returns a single printer by name
func (c *Client) GetPrinter(name string) (*Printer, error) {
	printers, err := c.GetPrinters()
	if err != nil {
		return nil, err
	}

	for _, p := range printers {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("printer %s not found", name)
}

// parsePrinterAttributes converts IPP attributes to a Printer struct
func (c *Client) parsePrinterAttributes(name string, attrs ipp.Attributes) Printer {
	printer := Printer{
		Name: name,
	}

	if v := getAttributeString(attrs, "printer-uri-supported"); v != "" {
		printer.URI = v
	}

	if v := getAttributeString(attrs, "printer-make-and-model"); v != "" {
		printer.MakeModel = v
	}

	if v := getAttributeString(attrs, "printer-location"); v != "" {
		printer.Location = v
	}

	if v := getAttributeString(attrs, "printer-info"); v != "" {
		printer.Info = v
	}

	if v, ok := getAttributeInt(attrs, "printer-state"); ok {
		printer.State = PrinterState(v)
	}

	if v, ok := getAttributeBool(attrs, "printer-is-shared"); ok {
		printer.IsShared = v
	}

	if v, ok := getAttributeBool(attrs, "printer-is-accepting-jobs"); ok {
		printer.IsAccepting = v
	}

	if v, ok := getAttributeBool(attrs, "color-supported"); ok {
		printer.ColorSupported = v
	}

	if sides := getAttributeStrings(attrs, "sides-supported"); len(sides) > 0 {
		printer.DuplexSupported = ParseDuplexSupport(sides)
	}

	if resolutions := getAttributeStrings(attrs, "printer-resolution-supported"); len(resolutions) > 0 {
		printer.Resolutions = ParseResolutions(resolutions)
	}

	if media := getAttributeStrings(attrs, "media-supported"); len(media) > 0 {
		printer.MediaSupported = media
	}

	if media := getAttributeStrings(attrs, "media-ready"); len(media) > 0 {
		printer.MediaReady = media
	}

	if v := getAttributeString(attrs, "media-default"); v != "" {
		printer.MediaDefault = v
	}

	return printer
}

// TestConnection tests the connection to CUPS
func (c *Client) TestConnection() error {
	_, err := c.cupsClient.GetPrinters([]string{"printer-name"})
	if err != nil {
		return fmt.Errorf("CUPS connection test failed: %w", err)
	}
	return nil
}

// Helper functions to extract values from IPP Attributes

func getAttributeString(attrs ipp.Attributes, name string) string {
	if attrList, ok := attrs[name]; ok && len(attrList) > 0 {
		if s, ok := attrList[0].Value.(string); ok {
			return s
		}
	}
	return ""
}

func getAttributeStrings(attrs ipp.Attributes, name string) []string {
	attrList, ok := attrs[name]
	if !ok || len(attrList) == 0 {
		return nil
	}

	var result []string
	for _, attr := range attrList {
		switch v := attr.Value.(type) {
		case string:
			result = append(result, v)
		case []string:
			result = append(result, v...)
		case ipp.Resolution:
			// Format resolution as string for parsing
			result = append(result, fmt.Sprintf("%dx%ddpi", v.Width, v.Height))
		}
	}
	return result
}

func getAttributeInt(attrs ipp.Attributes, name string) (int, bool) {
	if attrList, ok := attrs[name]; ok && len(attrList) > 0 {
		switch v := attrList[0].Value.(type) {
		case int:
			return v, true
		case int8:
			return int(v), true
		case int16:
			return int(v), true
		case int32:
			return int(v), true
		case int64:
			return int(v), true
		}
	}
	return 0, false
}

func getAttributeBool(attrs ipp.Attributes, name string) (bool, bool) {
	if attrList, ok := attrs[name]; ok && len(attrList) > 0 {
		if b, ok := attrList[0].Value.(bool); ok {
			return b, true
		}
	}
	return false, false
}
