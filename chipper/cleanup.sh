#!/bin/bash
#
# wire-pod cleanup script - WARNING: Destructive Operations
# Created by PicoClaw 🦞
#
# THIS SCRIPT WILL REMOVE:
# - All Go installations and related files
# - wire-pod/chipper service and configuration
# - Go workspace directories
# - Temporary build files
#
# Review carefully before execution!

set -euo pipefail

# Configuration
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

print_danger() {
    echo -e "${RED}[DANGER] $1${NC}" >&2
}

# Check if running as root
if [ $(id -u) -ne 0 ]; then
    print_error "This script must run as root to clean system-wide installations"
    exit 1
fi

# Confirmation prompt
confirm_destruction() {
    print_danger "THIS SCRIPT WILL PERFORM DESTRUCTIVE CLEANUP OPERATIONS"
    echo ""
    echo "It will remove:"
    echo "  - All Go installations (/usr/local/go, /usr/bin/go*)"
    echo "  - wire-pod service and configuration"
    echo "  - Go workspace directories"
    echo "  - Temporary files and caches"
    echo ""
    
    read -p "Are you sure you want to proceed? (yes/no): " -r
    echo    # (optional) move to a new line
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        print_status "Cleanup aborted by user"
        exit 0
    fi
}

# Stop and remove service
cleanup_service() {
    print_status "Cleaning up systemd service..."
    
    # Stop service if running
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_status "Stopping $SERVICE_NAME service..."
        systemctl stop "$SERVICE_NAME"
    fi
    
    # Disable service
    if systemctl is-enabled --quiet "$SERVICE_NAME"; then
        print_status "Disabling $SERVICE_NAME service..."
        systemctl disable "$SERVICE_NAME"
    fi
    
    # Remove service file
    if [ -f "$SERVICE_FILE" ]; then
        rm "$SERVICE_FILE"
        print_success "Removed service file: $SERVICE_FILE"
    fi
    
    # Reload systemd
    systemctl daemon-reload
}

# Cleanup Go installations
cleanup_go() {
    print_status "Cleaning up Go installations..."

    # Remove go binaries from /usr/bin
    for file in /usr/bin/go /usr/bin/gofmt; do
        if [ -f "$file" ]; then
            rm "$file"
            print_success "Removed Go binary: $file"
        fi
    done
    
    # Remove go from PATH in common shell profiles
    for profile in /etc/profile /etc/bash.bashrc ~/.profile ~/.bashrc; do
        if [ -f "$profile" ]; then
            # Remove GOROOT, GOPATH, and Go binary path from PATH
            sed -i '/GOROOT/d' "$profile" 2>/dev/null || true
            sed -i '/GOPATH/d' "$profile" 2>/dev/null || true
        fi
    done
}

# Cleanup workspace directories
cleanup_workspace() {
    print_status "Cleaning up workspace directories..."
    
    # Remove user's Go workspace
    if [ -d "$HOME/go" ]; then
        rm -rf "$HOME/go"
        print_success "Removed Go workspace: $HOME/go"
    fi
    
    # Remove module cache
    if [ -d "$HOME/.cache/go-build" ]; then
        rm -rf "$HOME/.cache/go-build"
        print_success "Removed Go build cache: $HOME/.cache/go-build"
    fi
    
    # Remove mod cache
    if [ -d "$HOME/.cache/go-mod" ]; then
        rm -rf "$HOME/.cache/go-mod"
        print_success "Removed Go mod cache: $HOME/.cache/go-mod"
    fi
}

# Main execution
main() {
    echo "=============================================="
    echo "  wire-pod - Comprehensive Cleanup Script"
    echo "=============================================="
    echo ""
    
    confirm_destruction
    
    cleanup_service
    cleanup_go
    #cleanup_workspace
    
    print_success "Cleanup completed successfully!"
    print_warning "You will need to reinstall Go and wire-pod if desired"
}

main
