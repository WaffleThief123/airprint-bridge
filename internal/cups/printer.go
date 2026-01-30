package cups

// Printer represents a CUPS printer with its capabilities
type Printer struct {
	Name        string
	URI         string
	MakeModel   string
	Location    string
	Info        string
	State       PrinterState
	IsShared    bool
	IsAccepting bool

	// Capabilities
	ColorSupported  bool
	DuplexSupported bool
	Resolutions     []int    // DPI values
	MediaSupported  []string // Paper sizes (e.g., "iso_a4_210x297mm")
	MediaReady      []string // Currently loaded paper
}

// PrinterState represents the CUPS printer state
type PrinterState int

const (
	PrinterStateIdle       PrinterState = 3
	PrinterStateProcessing PrinterState = 4
	PrinterStateStopped    PrinterState = 5
)

// String returns a human-readable state
func (s PrinterState) String() string {
	switch s {
	case PrinterStateIdle:
		return "idle"
	case PrinterStateProcessing:
		return "processing"
	case PrinterStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// IsAvailable returns true if the printer can accept jobs
func (p *Printer) IsAvailable() bool {
	return p.IsAccepting && p.State != PrinterStateStopped
}
