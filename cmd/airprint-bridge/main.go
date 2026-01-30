package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/WaffleThief123/airprint-bridge/internal/cups"
	"github.com/WaffleThief123/airprint-bridge/internal/daemon"
	"github.com/WaffleThief123/airprint-bridge/internal/media"
)

// Version information (set at build time)
var (
	version = "dev"
	commit  = "unknown"
)

// ConfigFile represents the YAML configuration file structure
type ConfigFile struct {
	CUPS struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"cups"`

	IPP struct {
		Port int `yaml:"port"`
	} `yaml:"ipp"`

	Monitor struct {
		PollInterval string `yaml:"poll_interval"`
	} `yaml:"monitor"`

	Avahi struct {
		ServiceDir string `yaml:"service_dir"`
		FilePrefix string `yaml:"file_prefix"`
	} `yaml:"avahi"`

	Printers struct {
		SharedOnly bool     `yaml:"shared_only"`
		Exclude    []string `yaml:"exclude"`
	} `yaml:"printers"`

	// Media overrides per printer
	Media []struct {
		Printer      string   `yaml:"printer"`       // Printer name to match
		Profile      string   `yaml:"profile"`       // Use a built-in profile (e.g., "zebra-4x6")
		Sizes        []string `yaml:"sizes"`         // Or specify custom sizes
		DefaultSize  string   `yaml:"default_size"`  // Default media size
	} `yaml:"media"`

	Log struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"log"`
}

func main() {
	// Command line flags
	var (
		configPath    = flag.String("config", "/etc/airprint-bridge/airprint-bridge.yaml", "path to config file")
		cupsHost      = flag.String("cups-host", "", "CUPS server host (default: localhost)")
		cupsPort      = flag.Int("cups-port", 0, "CUPS server port (default: 631)")
		ippPort       = flag.Int("ipp-port", 0, "IPP proxy server port (default: 8631)")
		pollInterval  = flag.String("poll-interval", "", "printer polling interval (default: 30s)")
		serviceDir    = flag.String("service-dir", "", "Avahi services directory")
		sharedOnly    = flag.Bool("shared-only", true, "only advertise shared printers")
		logLevel      = flag.String("log-level", "", "log level: debug, info, warn, error")
		logFormat     = flag.String("log-format", "", "log format: json, console")
		showVersion   = flag.Bool("version", false, "show version and exit")
		listPrinters  = flag.Bool("list-printers", false, "list available printers and exit")
		listProfiles  = flag.Bool("list-profiles", false, "list available media profiles and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("airprint-bridge version %s (commit %s)\n", version, commit)
		os.Exit(0)
	}

	// Start with defaults
	config := daemon.DefaultConfig()

	// Load config file if it exists
	if cfg, err := loadConfig(*configPath); err == nil {
		applyFileConfig(&config, cfg)
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config file: %v\n", err)
	}

	// Apply command line overrides
	if *cupsHost != "" {
		config.CUPSHost = *cupsHost
	}
	if *cupsPort != 0 {
		config.CUPSPort = *cupsPort
	}

	if *listPrinters {
		listAvailablePrinters(config.CUPSHost, config.CUPSPort)
		os.Exit(0)
	}

	if *listProfiles {
		listAvailableProfiles()
		os.Exit(0)
	}

	// Apply remaining command line overrides
	if *ippPort != 0 {
		config.IPPPort = *ippPort
	}
	if *pollInterval != "" {
		if d, err := time.ParseDuration(*pollInterval); err == nil {
			config.PollInterval = d
		}
	}
	if *serviceDir != "" {
		config.ServiceDir = *serviceDir
	}
	config.SharedOnly = *sharedOnly

	// Set up logging
	level := zerolog.InfoLevel
	if *logLevel != "" {
		level = parseLogLevel(*logLevel)
	}
	zerolog.SetGlobalLevel(level)

	var log zerolog.Logger
	if *logFormat == "json" {
		log = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			With().Timestamp().Logger()
	}

	// Create and run daemon
	d := daemon.New(config, log)
	if err := d.Run(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("daemon failed")
	}
}

func loadConfig(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func applyFileConfig(config *daemon.Config, cfg *ConfigFile) {
	if cfg.CUPS.Host != "" {
		config.CUPSHost = cfg.CUPS.Host
	}
	if cfg.CUPS.Port != 0 {
		config.CUPSPort = cfg.CUPS.Port
	}
	if cfg.IPP.Port != 0 {
		config.IPPPort = cfg.IPP.Port
	}
	if cfg.Monitor.PollInterval != "" {
		if d, err := time.ParseDuration(cfg.Monitor.PollInterval); err == nil {
			config.PollInterval = d
		}
	}
	if cfg.Avahi.ServiceDir != "" {
		config.ServiceDir = cfg.Avahi.ServiceDir
	}
	if cfg.Avahi.FilePrefix != "" {
		config.FilePrefix = cfg.Avahi.FilePrefix
	}
	config.SharedOnly = cfg.Printers.SharedOnly
	config.ExcludeList = cfg.Printers.Exclude

	// Apply media overrides
	for _, m := range cfg.Media {
		config.MediaOverrides = append(config.MediaOverrides, media.ConfigOverride{
			PrinterName:  m.Printer,
			ProfileName:  m.Profile,
			MediaSizes:   m.Sizes,
			DefaultMedia: m.DefaultSize,
		})
	}
}

func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func listAvailablePrinters(host string, port int) {
	client := cups.NewClient(host, port)
	printers, err := client.GetPrinters()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get printers from CUPS: %v\n", err)
		os.Exit(1)
	}

	if len(printers) == 0 {
		fmt.Println("No printers found in CUPS")
		return
	}

	fmt.Println("Available printers:")
	fmt.Println()
	for _, p := range printers {
		shared := ""
		if p.IsShared {
			shared = " [shared]"
		}
		fmt.Printf("  %s%s\n", p.Name, shared)
		if p.MakeModel != "" {
			fmt.Printf("    Model: %s\n", p.MakeModel)
		}
		if p.Location != "" {
			fmt.Printf("    Location: %s\n", p.Location)
		}
		if len(p.MediaSupported) > 0 {
			fmt.Printf("    Media: %d sizes available\n", len(p.MediaSupported))
		}
		fmt.Println()
	}

	fmt.Println("Use printer names in config file, e.g.:")
	fmt.Println()
	fmt.Println("  media:")
	fmt.Printf("    - printer: %s\n", printers[0].Name)
	fmt.Println("      profile: zebra-4x6")
}

func listAvailableProfiles() {
	registry := media.NewRegistry()
	profiles := registry.ListProfiles()

	fmt.Println("Available media profiles:")
	fmt.Println()
	for _, name := range profiles {
		p := registry.GetProfileByName(name)
		if p == nil {
			continue
		}
		fmt.Printf("  %s\n", name)
		fmt.Printf("    Auto-detects: %s\n", strings.Join(p.ModelMatch, ", "))
		fmt.Printf("    Sizes:\n")
		for _, size := range p.Sizes {
			if size.Description != "" {
				fmt.Printf("      - %-35s  %s\n", size.Name, size.Description)
			} else {
				fmt.Printf("      - %s\n", size.Name)
			}
		}
		fmt.Printf("    Default: %s\n", p.DefaultMedia)
		fmt.Println()
	}

	fmt.Println("Use in config file:")
	fmt.Println()
	fmt.Println("  media:")
	fmt.Println("    - printer: YOUR_PRINTER_NAME")
	fmt.Println("      profile: zebra-4x6")
}
