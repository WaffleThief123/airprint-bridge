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

	"github.com/cyra/airprint-cups-plugin/internal/daemon"
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

	Log struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"log"`
}

func main() {
	// Command line flags
	var (
		configPath  = flag.String("config", "/etc/airprint-bridge/airprint-bridge.yaml", "path to config file")
		cupsHost    = flag.String("cups-host", "", "CUPS server host (default: localhost)")
		cupsPort    = flag.Int("cups-port", 0, "CUPS server port (default: 631)")
		pollInterval = flag.String("poll-interval", "", "printer polling interval (default: 30s)")
		serviceDir  = flag.String("service-dir", "", "Avahi services directory")
		sharedOnly  = flag.Bool("shared-only", true, "only advertise shared printers")
		logLevel    = flag.String("log-level", "", "log level: debug, info, warn, error")
		logFormat   = flag.String("log-format", "", "log format: json, console")
		showVersion = flag.Bool("version", false, "show version and exit")
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
