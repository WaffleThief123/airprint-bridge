package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/WaffleThief123/airprint-bridge/internal/avahi"
	"github.com/WaffleThief123/airprint-bridge/internal/cups"
	"github.com/WaffleThief123/airprint-bridge/internal/ipp"
	"github.com/WaffleThief123/airprint-bridge/internal/media"
)

// Config holds the daemon configuration
type Config struct {
	CUPSHost       string
	CUPSPort       int
	IPPPort        int // Port for our IPP proxy server
	PollInterval   time.Duration
	ServiceDir     string
	FilePrefix     string
	SharedOnly     bool
	ExcludeList    []string
	MediaOverrides []media.ConfigOverride // Per-printer media overrides
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		CUPSHost:     "localhost",
		CUPSPort:     631,
		IPPPort:      8631,
		PollInterval: 30 * time.Second,
		ServiceDir:   "/etc/avahi/services",
		FilePrefix:   "airprint-",
		SharedOnly:   true,
		ExcludeList:  nil,
	}
}

// Daemon is the main AirPrint bridge daemon
type Daemon struct {
	config        Config
	cupsClient    *cups.Client
	avahiManager  *avahi.Manager
	mediaRegistry *media.Registry
	ippServers    map[string]*ipp.Server
	log           zerolog.Logger
}

// New creates a new daemon instance
func New(config Config, log zerolog.Logger) *Daemon {
	cupsClient := cups.NewClient(config.CUPSHost, config.CUPSPort)
	avahiManager := avahi.NewManager(
		config.ServiceDir,
		config.FilePrefix,
		config.IPPPort, // Use IPP proxy port, not CUPS port
		log,
	)

	// Initialize media registry with builtin profiles and apply config overrides
	mediaRegistry := media.NewRegistry()
	if len(config.MediaOverrides) > 0 {
		mediaRegistry.ApplyConfigOverrides(config.MediaOverrides)
	}

	return &Daemon{
		config:        config,
		cupsClient:    cupsClient,
		avahiManager:  avahiManager,
		mediaRegistry: mediaRegistry,
		ippServers:    make(map[string]*ipp.Server),
		log:           log.With().Str("component", "daemon").Logger(),
	}
}

// Run starts the daemon and blocks until shutdown
func (d *Daemon) Run(ctx context.Context) error {
	d.log.Info().
		Str("cups_host", d.config.CUPSHost).
		Int("cups_port", d.config.CUPSPort).
		Int("ipp_port", d.config.IPPPort).
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

	// Get initial printer list
	printers, err := d.cupsClient.GetPrinters()
	if err != nil {
		return fmt.Errorf("failed to get printers: %w", err)
	}
	d.log.Info().Int("count", len(printers)).Msg("discovered printers")

	// Start the IPP proxy server
	cupsProxy := ipp.NewCUPSProxy(d.config.CUPSHost, d.config.CUPSPort)

	// Determine local IP for advertising
	localIP := d.getLocalIP()
	d.log.Info().Str("local_ip", localIP).Msg("detected local IP")

	// Start IPP server
	listenAddr := fmt.Sprintf(":%d", d.config.IPPPort)

	// For now, use first printer (we can expand to multiple later)
	var printerConfig ipp.PrinterConfig
	if len(printers) > 0 {
		p := printers[0]

		// Get media from CUPS, then apply profile overrides
		cupsMedia := p.MediaReady
		if len(cupsMedia) == 0 {
			cupsMedia = p.MediaSupported
		}
		mediaList, mediaDefault := d.mediaRegistry.ApplyProfile(
			p.Name,
			p.MakeModel,
			cupsMedia,
			p.MediaDefault,
		)

		// Log whether we used a profile or CUPS defaults
		if profile := d.mediaRegistry.GetProfile(p.Name, p.MakeModel); profile != nil {
			d.log.Info().
				Str("printer", p.Name).
				Str("profile", profile.Name).
				Strs("media", mediaList).
				Str("default", mediaDefault).
				Msg("using media profile override")
		} else {
			d.log.Debug().
				Str("printer", p.Name).
				Strs("cups_media", cupsMedia).
				Str("cups_default", p.MediaDefault).
				Msg("using CUPS media configuration")
		}

		printerConfig = ipp.PrinterConfig{
			Name:           p.Name,
			MakeModel:      p.MakeModel,
			Location:       p.Location,
			Color:          p.ColorSupported,
			Duplex:         p.DuplexSupported,
			Resolutions:    p.Resolutions,
			MediaSupported: mediaList,
			MediaReady:     mediaList, // Use the same filtered list
			MediaDefault:   mediaDefault,
		}
	}

	ippServer := ipp.NewServer(listenAddr, cupsProxy, printerConfig, d.log)

	// Start IPP server in background
	go func() {
		if err := ippServer.ListenAndServe(); err != nil {
			d.log.Error().Err(err).Msg("IPP server failed")
		}
	}()
	d.log.Info().Int("port", d.config.IPPPort).Msg("started IPP proxy server")

	// Update Avahi service files
	if err := d.avahiManager.UpdatePrinters(printers, d.config.SharedOnly, d.config.ExcludeList); err != nil {
		d.log.Error().Err(err).Msg("failed to update service files")
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

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

// getLocalIP returns the local IP address for advertising
func (d *Daemon) getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}
