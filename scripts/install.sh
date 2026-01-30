#!/bin/sh
#
# AirPrint Bridge installer script
# Supports: Alpine, Debian/Ubuntu, Arch Linux, RHEL/CentOS/Fedora
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Defaults
PREFIX="${PREFIX:-/usr/local}"
BINDIR="${BINDIR:-$PREFIX/bin}"
CONFDIR="${CONFDIR:-/etc/airprint-bridge}"
BINARY_NAME="airprint-bridge"

# Detect script directory (where the binary should be)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

info() {
    printf "${GREEN}[INFO]${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1"
    exit 1
}

# Detect distribution
detect_distro() {
    # Check for macOS first
    if [ "$(uname -s)" = "Darwin" ]; then
        DISTRO="macos"
        DISTRO_TYPE="macos"
        return
    fi

    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO="$ID"
        DISTRO_FAMILY="$ID_LIKE"
    elif [ -f /etc/alpine-release ]; then
        DISTRO="alpine"
    elif [ -f /etc/debian_version ]; then
        DISTRO="debian"
    elif [ -f /etc/redhat-release ]; then
        DISTRO="rhel"
    elif [ -f /etc/arch-release ]; then
        DISTRO="arch"
    else
        DISTRO="unknown"
    fi

    # Normalize distro families
    case "$DISTRO" in
        ubuntu|debian|raspbian|linuxmint)
            DISTRO_TYPE="debian"
            ;;
        rhel|centos|fedora|rocky|almalinux|ol)
            DISTRO_TYPE="rhel"
            ;;
        alpine)
            DISTRO_TYPE="alpine"
            ;;
        arch|manjaro|endeavouros)
            DISTRO_TYPE="arch"
            ;;
        *)
            # Check ID_LIKE for family
            case "$DISTRO_FAMILY" in
                *debian*|*ubuntu*)
                    DISTRO_TYPE="debian"
                    ;;
                *rhel*|*fedora*|*centos*)
                    DISTRO_TYPE="rhel"
                    ;;
                *arch*)
                    DISTRO_TYPE="arch"
                    ;;
                *)
                    DISTRO_TYPE="unknown"
                    ;;
            esac
            ;;
    esac
}

# Check for root
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        error "This script must be run as root (try: sudo $0)"
    fi
}

# Find the binary
find_binary() {
    # Check common locations
    if [ -f "$SCRIPT_DIR/$BINARY_NAME" ]; then
        BINARY_PATH="$SCRIPT_DIR/$BINARY_NAME"
    elif [ -f "./$BINARY_NAME" ]; then
        BINARY_PATH="./$BINARY_NAME"
    elif [ -f "$SCRIPT_DIR/../$BINARY_NAME" ]; then
        BINARY_PATH="$SCRIPT_DIR/../$BINARY_NAME"
    else
        error "Cannot find $BINARY_NAME binary. Make sure it's in the same directory as this script."
    fi
    info "Found binary at $BINARY_PATH"
}

# Install dependencies
install_deps() {
    info "Installing dependencies..."

    case "$DISTRO_TYPE" in
        alpine)
            apk add --no-cache cups cups-filters avahi dbus
            ;;
        debian)
            apt-get update
            apt-get install -y cups cups-filters avahi-daemon
            ;;
        rhel)
            if command -v dnf >/dev/null 2>&1; then
                dnf install -y cups cups-filters avahi
            else
                yum install -y cups cups-filters avahi
            fi
            ;;
        arch)
            pacman -S --noconfirm --needed cups cups-filters avahi
            ;;
        macos)
            # macOS has CUPS built-in, just need to ensure it's running
            # Avahi is not used on macOS (uses Bonjour/mDNSResponder instead)
            info "macOS has CUPS built-in"
            info "Checking if CUPS is enabled..."
            if ! cupsctl 2>/dev/null | grep -q "_share_printers=1"; then
                warn "Printer sharing may not be enabled in CUPS"
                warn "Enable in System Preferences > Sharing > Printer Sharing"
            fi
            ;;
        *)
            warn "Unknown distribution, skipping dependency installation"
            warn "Please manually install: cups, cups-filters, avahi"
            ;;
    esac
}

# Install binary
install_binary() {
    info "Installing binary to $BINDIR..."
    mkdir -p "$BINDIR"
    cp "$BINARY_PATH" "$BINDIR/$BINARY_NAME"
    chmod 755 "$BINDIR/$BINARY_NAME"
}

# Install config
install_config() {
    info "Installing configuration to $CONFDIR..."
    mkdir -p "$CONFDIR"

    if [ -f "$CONFDIR/airprint-bridge.yaml" ]; then
        warn "Config file already exists, not overwriting"
        warn "New config saved to $CONFDIR/airprint-bridge.yaml.new"
        cat > "$CONFDIR/airprint-bridge.yaml.new" << 'CONFIGEOF'
# AirPrint Bridge Configuration

cups:
  host: localhost
  port: 631

ipp:
  port: 8631

monitor:
  poll_interval: 30s

avahi:
  service_dir: /etc/avahi/services
  file_prefix: airprint-

printers:
  shared_only: true
  exclude: []

media: []

log:
  level: info
  format: console
CONFIGEOF
    else
        cat > "$CONFDIR/airprint-bridge.yaml" << 'CONFIGEOF'
# AirPrint Bridge Configuration

cups:
  host: localhost
  port: 631

ipp:
  port: 8631

monitor:
  poll_interval: 30s

avahi:
  service_dir: /etc/avahi/services
  file_prefix: airprint-

printers:
  shared_only: true
  exclude: []

media: []

log:
  level: info
  format: console
CONFIGEOF
    fi
}

# Install service files
install_service() {
    info "Installing service files..."

    case "$DISTRO_TYPE" in
        alpine)
            install_openrc_service
            ;;
        debian|rhel|arch)
            install_systemd_service
            ;;
        macos)
            install_launchd_service
            ;;
        *)
            warn "Unknown init system, skipping service installation"
            warn "You'll need to manually set up the service"
            ;;
    esac
}

install_openrc_service() {
    cat > /etc/init.d/airprint-bridge << 'SERVICEEOF'
#!/sbin/openrc-run

name="airprint-bridge"
description="AirPrint Bridge for CUPS"
command="/usr/local/bin/airprint-bridge"
command_args="--config /etc/airprint-bridge/airprint-bridge.yaml"
command_background=true
pidfile="/run/${RC_SVCNAME}.pid"
output_log="/var/log/airprint-bridge.log"
error_log="/var/log/airprint-bridge.log"

depend() {
    need cupsd avahi-daemon
    after cupsd avahi-daemon
}

start_pre() {
    checkpath --file --owner root:root --mode 0644 "$output_log"
}
SERVICEEOF
    chmod 755 /etc/init.d/airprint-bridge
    info "OpenRC service installed"
    info "Enable with: rc-update add airprint-bridge default"
    info "Start with:  rc-service airprint-bridge start"
}

install_systemd_service() {
    cat > /etc/systemd/system/airprint-bridge.service << SERVICEEOF
[Unit]
Description=AirPrint Bridge for CUPS
Documentation=https://github.com/WaffleThief123/airprint-bridge
After=network.target cups.service avahi-daemon.service
Requires=cups.service avahi-daemon.service

[Service]
Type=simple
ExecStart=$BINDIR/airprint-bridge --config /etc/airprint-bridge/airprint-bridge.yaml
ExecReload=/bin/kill -HUP \$MAINPID
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/etc/avahi/services
PrivateTmp=true

[Install]
WantedBy=multi-user.target
SERVICEEOF
    systemctl daemon-reload
    info "systemd service installed"
    info "Enable with: systemctl enable airprint-bridge"
    info "Start with:  systemctl start airprint-bridge"
}

install_launchd_service() {
    PLIST_PATH="/Library/LaunchDaemons/com.github.wafflethief123.airprint-bridge.plist"
    cat > "$PLIST_PATH" << SERVICEEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.github.wafflethief123.airprint-bridge</string>
    <key>ProgramArguments</key>
    <array>
        <string>$BINDIR/airprint-bridge</string>
        <string>--config</string>
        <string>/etc/airprint-bridge/airprint-bridge.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/airprint-bridge.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/airprint-bridge.log</string>
</dict>
</plist>
SERVICEEOF
    chmod 644 "$PLIST_PATH"
    info "launchd service installed"
    info "Load with:   sudo launchctl load $PLIST_PATH"
    info "Start with:  sudo launchctl start com.github.wafflethief123.airprint-bridge"
}

# Enable required services
enable_services() {
    info "Enabling required services..."

    case "$DISTRO_TYPE" in
        alpine)
            rc-update add dbus default 2>/dev/null || true
            rc-update add avahi-daemon default 2>/dev/null || true
            rc-update add cupsd default 2>/dev/null || true
            rc-service dbus start 2>/dev/null || true
            rc-service avahi-daemon start 2>/dev/null || true
            rc-service cupsd start 2>/dev/null || true
            ;;
        debian|rhel|arch)
            systemctl enable --now avahi-daemon 2>/dev/null || true
            systemctl enable --now cups 2>/dev/null || true
            ;;
        macos)
            # CUPS and mDNSResponder are built into macOS
            # Just ensure CUPS is started
            launchctl load /System/Library/LaunchDaemons/org.cups.cupsd.plist 2>/dev/null || true
            ;;
    esac
}

# Uninstall
uninstall() {
    info "Uninstalling AirPrint Bridge..."

    # Stop service
    case "$DISTRO_TYPE" in
        alpine)
            rc-service airprint-bridge stop 2>/dev/null || true
            rc-update del airprint-bridge default 2>/dev/null || true
            rm -f /etc/init.d/airprint-bridge
            ;;
        debian|rhel|arch)
            systemctl stop airprint-bridge 2>/dev/null || true
            systemctl disable airprint-bridge 2>/dev/null || true
            rm -f /etc/systemd/system/airprint-bridge.service
            systemctl daemon-reload
            ;;
        macos)
            PLIST_PATH="/Library/LaunchDaemons/com.github.wafflethief123.airprint-bridge.plist"
            launchctl stop com.github.wafflethief123.airprint-bridge 2>/dev/null || true
            launchctl unload "$PLIST_PATH" 2>/dev/null || true
            rm -f "$PLIST_PATH"
            ;;
    esac

    # Remove binary
    rm -f "$BINDIR/$BINARY_NAME"

    # Remove Avahi service files created by the daemon (Linux only)
    rm -f /etc/avahi/services/airprint-*.service 2>/dev/null || true

    info "Uninstall complete"
    info "Config files preserved in $CONFDIR (remove manually if desired)"
}

# Show usage
usage() {
    cat << EOF
AirPrint Bridge Installer

Usage: $0 [OPTIONS] [COMMAND]

Commands:
    install     Install AirPrint Bridge (default)
    uninstall   Remove AirPrint Bridge
    deps        Install only dependencies

Options:
    -h, --help      Show this help message
    -n, --no-deps   Skip dependency installation
    -s, --no-service Skip service file installation

Environment variables:
    PREFIX      Installation prefix (default: /usr/local)
    BINDIR      Binary directory (default: \$PREFIX/bin)
    CONFDIR     Config directory (default: /etc/airprint-bridge)

Examples:
    $0                      # Install with defaults
    $0 --no-deps install    # Install without dependencies
    $0 uninstall            # Uninstall
    BINDIR=/usr/bin $0      # Install binary to /usr/bin

EOF
    exit 0
}

# Parse arguments
SKIP_DEPS=0
SKIP_SERVICE=0
COMMAND="install"

while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            ;;
        -n|--no-deps)
            SKIP_DEPS=1
            shift
            ;;
        -s|--no-service)
            SKIP_SERVICE=1
            shift
            ;;
        install|uninstall|deps)
            COMMAND="$1"
            shift
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Main
main() {
    check_root
    detect_distro

    info "Detected distribution: $DISTRO ($DISTRO_TYPE)"

    case "$COMMAND" in
        install)
            find_binary
            [ "$SKIP_DEPS" -eq 0 ] && install_deps
            install_binary
            install_config
            [ "$SKIP_SERVICE" -eq 0 ] && install_service
            enable_services

            echo ""
            info "Installation complete!"
            info ""
            info "Next steps:"
            info "  1. Edit config: $CONFDIR/airprint-bridge.yaml"
            info "  2. Check printers: $BINDIR/$BINARY_NAME --list-printers"
            info "  3. Start service (see above for commands)"
            ;;
        uninstall)
            uninstall
            ;;
        deps)
            install_deps
            info "Dependencies installed"
            ;;
    esac
}

main
