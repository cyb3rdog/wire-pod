#!/bin/bash
#
# wire-pod chipper - Production Build & Deployment Script
# Created by PicoClaw 🦞
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Configuration
BINARY_NAME="wire-pod"
INSTALL_DIR="/usr/bin"
SERVICE_NAME="wire-pod"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[*] $1${NC}"
}

print_success() {
    echo -e "${GREEN}[+] $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}[!] $1${NC}"
}

print_error() {
    echo -e "${RED}[ERROR] $1${NC}" >&2
}

# Check if running as root (needed for installation)
is_root() {
    [ $(id -u) -eq 0 ]
}

# Check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    GO_VERSION=$($(which go) version | awk '{print $3}' | sed 's/go//')
    echo "  Go version: $GO_VERSION"
    
    # Check minimum version
    if [[ "$(printf '%s\n' "1.19" "$GO_VERSION" | sort -V | head -n1)" != "1.19" ]]; then
        print_error "Go 1.19+ required, found $GO_VERSION"
        exit 1
    fi
}

# Build the binary
build_binary() {
    print_status "Building $BINARY_NAME..."
    
    # Get commit hash
    if command -v git &> /dev/null && [ -d ".git" ]; then
        COMMIT_HASH="$(git rev-parse --short HEAD)"
    else
        COMMIT_HASH="unknown"
    fi
    
    # Clean previous build
    rm -f "$BINARY_NAME"
    
    # Source config
    if [[ -f source.sh ]]; then 
        source source.sh 
    fi
    
    # Build for leopard (STT_SERVICE)
    go build -v -tags "${GOTAGS:-nolibopusfile}" -ldflags "${GOLDFLAGS:--X 'github.com/kercre123/wire-pod/chipper/pkg/vars.CommitSHA=${COMMIT_HASH}' -s -w}" -o "$BINARY_NAME" ./cmd/leopard
    
    if [ ! -f "$BINARY_NAME" ]; then
        print_error "Build failed - no binary produced"
        exit 1
    fi
    
    SIZE=$(du -h "$BINARY_NAME" | cut -f1)
    print_success "Build successful: $BINARY_NAME ($SIZE)"
}

# Create systemd service file
create_service_file() {
    print_status "Creating systemd service file..."
    
    cat > "/tmp/${SERVICE_NAME}.service" << 'EOF'
[Unit]
Description=wire-pod chipper STT Service
Wants=network-online.target
After=network.target network-online.target

[Service]
Type=simple
WorkingDirectory=/etc/wire-pod
ExecStartPre=/usr/sbin/iw dev wlan0 set power_save off
ExecStartPre=/bin/systemctl restart zramswap
ExecStart=/usr/bin/wire-pod
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

Environment="GOMAXPROCS=1"

[Install]
WantedBy=multi-user.target
EOF
    # Create config directory
    mkdir -p /etc/wire-pod
    
    # Move service file to final location
    mv "/tmp/${SERVICE_NAME}.service" "$SERVICE_FILE"
    chmod 644 "$SERVICE_FILE"
    
    print_success "Service file created at $SERVICE_FILE"
}

# Install binary
install_binary() {
    print_status "Installing binary to $INSTALL_DIR..."
    
    # Stop service if running
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_status "Stopping $SERVICE_NAME service..."
        systemctl stop "$SERVICE_NAME"
    fi
    
    # Copy binary
    cp "$BINARY_NAME" "$INSTALL_DIR/"
    chmod 755 "$INSTALL_DIR/$BINARY_NAME"
    
    # Reload systemd
    systemctl daemon-reload
    
    print_success "Binary installed to $INSTALL_DIR/$BINARY_NAME"
}

# Enable and start service
manage_service() {
    print_status "Configuring service..."
    
    # Enable service
    systemctl enable "$SERVICE_NAME" 2>/dev/null || true
    
    # Start service
    systemctl start "$SERVICE_NAME"
    
    # Check status
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "$SERVICE_NAME service is running"
        
        # Show service status
        echo ""
        systemctl status "$SERVICE_NAME" --no-pager -l | head -20
    else
        print_error "$SERVICE_NAME service failed to start"
        echo "Check logs with: journalctl -u $SERVICE_NAME -f"
        exit 1
    fi
}

# Main execution
main() {
    echo "=============================================="
    echo "  wire-pod chipper - Production Build & Deploy"
    echo "=============================================="
    echo ""
    
    check_prerequisites
    build_binary
    
    # Only perform installation steps if running as root
    if is_root; then
        create_service_file
        install_binary
        manage_service
        
        print_success "Deployment completed successfully! ✅"
        echo ""
        echo "Service management commands:"
        echo "  Start:    systemctl start $SERVICE_NAME"
        echo "  Stop:     systemctl stop $SERVICE_NAME"
        echo "  Status:   systemctl status $SERVICE_NAME"
        echo "  Logs:     journalctl -u $SERVICE_NAME -f"
    else
        print_warning "Build completed, but installation requires root privileges."
        print_warning "Run with sudo to install and configure the service:"
        echo "  sudo ./build-release.sh"
        print_success "Binary ready: $SCRIPT_DIR/$BINARY_NAME"
    fi
}

main
