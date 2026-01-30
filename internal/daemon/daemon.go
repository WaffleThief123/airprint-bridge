package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/cyra/airprint-cups-plugin/internal/avahi"
	"github.com/cyra/airprint-cups-plugin/internal/cups"
)

// Config holds the daemon configuration
type Config struct {
	CUPSHost     string
	CUPSPort     int
	PollInterval time.Duration
	ServiceDir   string
	FilePrefix   string
	SharedOnly   bool
	ExcludeList  []string
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		CUPSHost:     "localhost",
		CUPSPort:     631,
		PollInterval: 30 * time.Second,
		ServiceDir:   "/etc/avahi/services",
		FilePrefix:   "airprint-",
		SharedOnly:   true,
		ExcludeList:  nil,
	}
}

// Daemon is the main AirPrint bridge daemon
type Daemon struct {
	config       Config
	cupsClient   *cups.Client
	avahiManager *avahi.Manager
	log          zerolog.Logger
}

// New creates a new daemon instance
func New(config Config, log zerolog.Logger) *Daemon {
	cupsClient := cups.NewClient(config.CUPSHost, config.CUPSPort)
	avahiManager := avahi.NewManager(
		config.ServiceDir,
		config.FilePrefix,
		config.CUPSPort,
		log,
	)

	return &Daemon{
		config:       config,
		cupsClient:   cupsClient,
		avahiManager: avahiManager,
		log:          log.With().Str("component", "daemon").Logger(),
	}
}

// Run starts the daemon and blocks until shutdown
func (d *Daemon) Run(ctx context.Context) error {
	d.log.Info().
		Str("cups_host", d.config.CUPSHost).
		Int("cups_port", d.config.CUPSPort).
		Dur("poll_interval", d.config.PollInterval).
		Str("service_dir", d.config.ServiceDir).
		Bool("shared_only", d.config.SharedOnly).
		Msg("starting AirPrint bridge daemon")

	// Verify CUPS connection
	if err := d.cupsClient.TestConnection(); err != nil {
		return fmt.Errorf("cannot connect to CUPS: %w", err)
	}
	d.log.Info().Msg("connected to CUPS")

	// Verify service directory exists and is writable
	if err := d.verifyServiceDir(); err != nil {
		return err
	}

	// Discover any existing service files from previous runs
	if err := d.avahiManager.DiscoverExisting(); err != nil {
		d.log.Warn().Err(err).Msg("failed to discover existing service files")
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Initial printer sync
	if err := d.syncPrinters(); err != nil {
		d.log.Error().Err(err).Msg("initial printer sync failed")
		// Continue anyway, will retry on next poll
	}

	// Main loop
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.Info().Msg("context cancelled, shutting down")
			return d.shutdown()

		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				d.log.Info().Msg("received SIGHUP, reloading")
				if err := d.syncPrinters(); err != nil {
					d.log.Error().Err(err).Msg("reload failed")
				}
			case syscall.SIGTERM, syscall.SIGINT:
				d.log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
				return d.shutdown()
			}

		case <-ticker.C:
			if err := d.syncPrinters(); err != nil {
				d.log.Error().Err(err).Msg("printer sync failed")
			}
		}
	}
}

// syncPrinters fetches printers from CUPS and updates Avahi service files
func (d *Daemon) syncPrinters() error {
	printers, err := d.cupsClient.GetPrinters()
	if err != nil {
		return fmt.Errorf("failed to get printers: %w", err)
	}

	d.log.Debug().Int("count", len(printers)).Msg("fetched printers from CUPS")

	return d.avahiManager.UpdatePrinters(printers, d.config.SharedOnly, d.config.ExcludeList)
}

// shutdown performs cleanup and returns
func (d *Daemon) shutdown() error {
	d.log.Info().Msg("cleaning up service files")
	if err := d.avahiManager.Cleanup(); err != nil {
		d.log.Error().Err(err).Msg("cleanup failed")
		return err
	}
	d.log.Info().Msg("shutdown complete")
	return nil
}

// verifyServiceDir checks that the Avahi service directory exists and is writable
func (d *Daemon) verifyServiceDir() error {
	info, err := os.Stat(d.config.ServiceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("service directory does not exist: %s", d.config.ServiceDir)
		}
		return fmt.Errorf("cannot access service directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("service directory is not a directory: %s", d.config.ServiceDir)
	}

	// Try to create and remove a test file
	testFile := d.config.ServiceDir + "/.airprint-bridge-test"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("service directory is not writable: %w", err)
	}
	os.Remove(testFile)

	return nil
}
