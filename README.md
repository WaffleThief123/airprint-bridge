# AirPrint Bridge for CUPS

A lightweight Go daemon that exposes CUPS printers as AirPrint-compatible printers, enabling iOS and macOS devices to discover and print to them.

## How It Works

```
iOS Device → mDNS Discovery → Avahi (reads service files)
iOS Device → IPP Print Job → AirPrint Bridge (port 8631) → CUPS → Printer
Daemon → Monitors CUPS → Generates Avahi service files
```

The daemon:
1. Queries CUPS for available printers and their capabilities
2. Runs an IPP proxy server that iOS/macOS connects to
3. Generates Avahi service files with proper AirPrint TXT records
4. Forwards print jobs to CUPS
5. Monitors for printer changes and automatically updates advertisements

The IPP proxy approach avoids issues with CUPS access controls, TLS configuration, and hostname resolution that can prevent direct iOS→CUPS printing.

## Requirements

- CUPS (cupsd)
- cups-filters (for URF support)
- Avahi (avahi-daemon)
- D-Bus (required by Avahi)

## Installation

### Quick Install (Recommended)

Download the latest release and run the install script:

```bash
# Download and extract
tar -xzf airprint-bridge-*-linux-amd64.tar.gz
cd airprint-bridge-*

# Install (as root)
sudo ./install.sh
```

The install script automatically:
- Detects your OS (Alpine, Debian/Ubuntu, Arch, RHEL/CentOS/Fedora, macOS)
- Installs dependencies (cups, cups-filters, avahi on Linux)
- Installs the binary and config file
- Sets up the appropriate service (OpenRC, systemd, or launchd)

#### Install Script Options

```bash
# Install without dependencies (if already installed)
sudo ./install.sh --no-deps

# Install without service files
sudo ./install.sh --no-service

# Uninstall
sudo ./install.sh uninstall

# Show help
./install.sh --help
```

### Starting the Service

**Alpine (OpenRC):**
```bash
rc-update add airprint-bridge default
rc-service airprint-bridge start
```

**Debian/Ubuntu/Arch/RHEL (systemd):**
```bash
systemctl enable airprint-bridge
systemctl start airprint-bridge
```

**macOS (launchd):**
```bash
sudo launchctl load /Library/LaunchDaemons/com.github.wafflethief123.airprint-bridge.plist
sudo launchctl start com.github.wafflethief123.airprint-bridge
```

### Building from Source

```bash
# Install Go 1.21+
# Clone the repo
git clone https://github.com/WaffleThief123/airprint-bridge.git
cd airprint-bridge

# Build
make deps
make build

# Create distribution package
make dist
```

## Configuration

Edit `/etc/airprint-bridge/airprint-bridge.yaml`:

```yaml
cups:
  host: localhost
  port: 631

ipp:
  port: 8631  # Port iOS/macOS connects to

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

## Media Size Profiles

By default, media sizes are queried from CUPS. For label printers and other specialty devices, you can override with built-in profiles or custom sizes.

### Built-in Profiles

| Profile | Printers | Sizes |
|---------|----------|-------|
| `zebra-4x6` | Zebra ZPL printers | 4x6, 4x4, 4x3, 4x2, 2.25x1.25 inch |
| `dymo-labelwriter` | DYMO LabelWriter | Shipping, address, return address labels |
| `brother-ql` | Brother QL series | 62x100mm, 62x29mm, 29x90mm, etc. |
| `rollo` | Rollo thermal | 4x6, 4x4, 4x2 inch |

Profiles are auto-detected by matching printer make/model. You can also assign them explicitly.

### Using a Profile

```yaml
media:
  - printer: ZTC_ZP_450
    profile: zebra-4x6
```

### Custom Media Sizes

```yaml
media:
  - printer: My_Label_Printer
    sizes:
      - oe_4x6-label_4x6in
      - oe_4x4-label_4x4in
      - oe_2x1-label_2x1in
    default_size: oe_4x6-label_4x6in
```

### Listing Printers and Profiles

```bash
# List available printers from CUPS
airprint-bridge --list-printers

# List available media profiles
airprint-bridge --list-profiles
```

### Finding Media Size Names

Query your printer's supported sizes from CUPS:

```bash
ipptool -tv ipp://localhost/printers/YOUR_PRINTER get-printer-attributes.test | grep media
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

### Test IPP Server

```bash
curl -v http://localhost:8631/
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

1. Check Avahi is running:
   - Alpine: `rc-service avahi-daemon status`
   - systemd: `systemctl status avahi-daemon`
2. Check service files exist: `ls /etc/avahi/services/airprint-*`
3. Check firewall allows mDNS (UDP 5353) and IPP (TCP 8631)
4. Verify printer is shared in CUPS

### Check daemon logs

```bash
# View logs (systemd)
journalctl -u airprint-bridge -f

# View logs (Alpine)
tail -f /var/log/airprint-bridge.log

# Or run in foreground with debug logging
airprint-bridge --log-level debug --log-format console
```

### Wrong media sizes showing

1. Check what CUPS reports: `ipptool -tv ipp://localhost/printers/PRINTER get-printer-attributes.test | grep media`
2. Add a media profile override in config (see Media Size Profiles above)
3. Restart the daemon

### Reload after config changes

```bash
# Alpine (OpenRC)
rc-service airprint-bridge restart

# systemd
systemctl restart airprint-bridge
```

## Signals

- `SIGTERM` / `SIGINT`: Graceful shutdown (cleans up service files)
- `SIGHUP`: Reload configuration and resync printers

## License

MIT
