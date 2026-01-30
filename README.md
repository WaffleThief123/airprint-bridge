# AirPrint Bridge for CUPS

A lightweight Go daemon that exposes CUPS printers as AirPrint-compatible printers, enabling iOS and macOS devices to discover and print to them.

## How It Works

```
iOS Device → mDNS Discovery → Avahi (reads service files)
iOS Device → IPP Print Job → CUPS (port 631) → cups-filters (URF→PDF) → Printer
Daemon → Monitors CUPS → Generates/Updates Avahi service files
```

The daemon:
1. Queries CUPS for available printers and their capabilities
2. Generates Avahi service files with proper AirPrint TXT records
3. Monitors for printer changes and automatically updates advertisements

CUPS handles the actual IPP printing directly - no proxy needed. The `cups-filters` package provides URF→PDF conversion via `rastertopdf`.

## Requirements

- Go 1.21+ (for building)
- CUPS (cupsd)
- cups-filters (for URF support)
- Avahi (avahi-daemon)
- D-Bus (required by Avahi)

### Alpine Linux

```bash
apk add cups cups-filters avahi dbus
rc-service dbus start
rc-service avahi-daemon start
rc-service cupsd start
```

## Building

```bash
# Download dependencies
make deps

# Build
make build

# Or build a static binary
make build-static
```

## Installation

```bash
# Install binary, config, and service
sudo make install

# Enable at boot
sudo rc-update add airprint-bridge default

# Start the service
sudo rc-service airprint-bridge start
```

## Configuration

Edit `/etc/airprint-bridge/airprint-bridge.yaml`:

```yaml
cups:
  host: localhost
  port: 631

monitor:
  poll_interval: 30s

avahi:
  service_dir: /etc/avahi/services
  file_prefix: airprint-

printers:
  shared_only: true
  exclude:
    - PDF_Printer
```

## Verification

### Check mDNS Advertisement

```bash
avahi-browse -r _ipp._tcp
```

You should see your printers listed with AirPrint TXT records including `URF=`, `Color=`, `Duplex=`, etc.

### Check Service Files

```bash
ls -la /etc/avahi/services/airprint-*.service
cat /etc/avahi/services/airprint-YourPrinter.service
```

### Test from iOS

1. Open any app with print functionality
2. Tap Share → Print
3. Your CUPS printers should appear
4. Print a test page

### Verify URF Support

```bash
# Check CUPS MIME types
grep -i urf /etc/cups/mime.types

# Should include:
# image/urf              urf string(0,UNIRAST)
```

## Troubleshooting

### Printers not appearing on iOS

1. Check Avahi is running: `rc-service avahi-daemon status`
2. Check service files exist: `ls /etc/avahi/services/airprint-*`
3. Check firewall allows mDNS (UDP 5353) and IPP (TCP 631)
4. Verify printer is shared in CUPS

### Check daemon logs

```bash
# View logs
tail -f /var/log/airprint-bridge.log

# Or run in foreground with debug logging
airprint-bridge --log-level debug --log-format console
```

### Reload after config changes

```bash
# Send SIGHUP to reload
rc-service airprint-bridge reload

# Or restart
rc-service airprint-bridge restart
```

## Signals

- `SIGTERM` / `SIGINT`: Graceful shutdown (cleans up service files)
- `SIGHUP`: Reload configuration and resync printers

## License

MIT
