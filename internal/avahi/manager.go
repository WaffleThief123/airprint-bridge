package avahi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/cyra/airprint-cups-plugin/internal/airprint"
	"github.com/cyra/airprint-cups-plugin/internal/cups"
)

// Manager handles the lifecycle of Avahi service files
type Manager struct {
	serviceDir string
	filePrefix string
	cupsPort   int
	log        zerolog.Logger
	mu         sync.Mutex

	// Track which files we've created
	managedFiles map[string]bool
}

// NewManager creates a new Avahi service file manager
func NewManager(serviceDir, filePrefix string, cupsPort int, log zerolog.Logger) *Manager {
	return &Manager{
		serviceDir:   serviceDir,
		filePrefix:   filePrefix,
		cupsPort:     cupsPort,
		log:          log.With().Str("component", "avahi-manager").Logger(),
		managedFiles: make(map[string]bool),
	}
}

// UpdatePrinters updates service files based on current CUPS printers
func (m *Manager) UpdatePrinters(printers []cups.Printer, sharedOnly bool, excludeList []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Build exclude map for quick lookup
	exclude := make(map[string]bool)
	for _, name := range excludeList {
		exclude[strings.ToLower(name)] = true
	}

	// Track which printers we see this round
	currentPrinters := make(map[string]bool)

	for _, printer := range printers {
		// Skip excluded printers
		if exclude[strings.ToLower(printer.Name)] {
			m.log.Debug().Str("printer", printer.Name).Msg("skipping excluded printer")
			continue
		}

		// Skip non-shared printers if configured
		if sharedOnly && !printer.IsShared {
			m.log.Debug().Str("printer", printer.Name).Msg("skipping non-shared printer")
			continue
		}

		// Skip printers that aren't accepting jobs
		if !printer.IsAccepting {
			m.log.Debug().Str("printer", printer.Name).Msg("skipping printer not accepting jobs")
			continue
		}

		filename := ServiceFileName(m.filePrefix, printer.Name)
		currentPrinters[filename] = true

		if err := m.createOrUpdateService(&printer); err != nil {
			m.log.Error().Err(err).Str("printer", printer.Name).Msg("failed to update service file")
			// Continue with other printers
		}
	}

	// Remove service files for printers that no longer exist
	for filename := range m.managedFiles {
		if !currentPrinters[filename] {
			m.log.Info().Str("file", filename).Msg("removing orphaned service file")
			if err := m.removeServiceFile(filename); err != nil {
				m.log.Error().Err(err).Str("file", filename).Msg("failed to remove service file")
			}
			delete(m.managedFiles, filename)
		}
	}

	return nil
}

// createOrUpdateService creates or updates a service file for a printer
func (m *Manager) createOrUpdateService(printer *cups.Printer) error {
	// Generate TXT records
	txtRecords := airprint.NewTXTRecords(printer)

	// Generate service file content
	content, err := GenerateServiceFile(printer.Name, m.cupsPort, txtRecords.All())
	if err != nil {
		return fmt.Errorf("failed to generate service file: %w", err)
	}

	filename := ServiceFileName(m.filePrefix, printer.Name)
	filepath := filepath.Join(m.serviceDir, filename)

	// Check if file exists and has same content
	existing, err := os.ReadFile(filepath)
	if err == nil && string(existing) == string(content) {
		m.log.Debug().Str("printer", printer.Name).Msg("service file unchanged")
		return nil
	}

	// Write atomically: write to temp file, then rename
	if err := m.atomicWrite(filepath, content); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	m.managedFiles[filename] = true
	m.log.Info().
		Str("printer", printer.Name).
		Str("file", filename).
		Bool("color", printer.ColorSupported).
		Bool("duplex", printer.DuplexSupported).
		Msg("updated service file")

	return nil
}

// atomicWrite writes content to a file atomically using a temp file and rename
func (m *Manager) atomicWrite(filepath string, content []byte) error {
	// Create temp file in the same directory
	tmpPath := filepath + ".tmp"

	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, filepath); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// removeServiceFile removes a service file
func (m *Manager) removeServiceFile(filename string) error {
	filepath := filepath.Join(m.serviceDir, filename)
	if err := os.Remove(filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	return nil
}

// Cleanup removes all managed service files
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for filename := range m.managedFiles {
		if err := m.removeServiceFile(filename); err != nil {
			m.log.Error().Err(err).Str("file", filename).Msg("failed to remove service file during cleanup")
			lastErr = err
		} else {
			m.log.Info().Str("file", filename).Msg("removed service file")
		}
	}

	m.managedFiles = make(map[string]bool)
	return lastErr
}

// DiscoverExisting finds and tracks service files we may have created previously
func (m *Manager) DiscoverExisting() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pattern := filepath.Join(m.serviceDir, m.filePrefix+"*.service")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob service files: %w", err)
	}

	for _, match := range matches {
		filename := filepath.Base(match)
		m.managedFiles[filename] = true
		m.log.Debug().Str("file", filename).Msg("discovered existing service file")
	}

	if len(matches) > 0 {
		m.log.Info().Int("count", len(matches)).Msg("discovered existing service files")
	}

	return nil
}
