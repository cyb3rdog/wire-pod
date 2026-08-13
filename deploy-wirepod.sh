#!/bin/bash
# RPi WirePod Deploy Protocol - Host Execution (sets RPI_IP env or fail)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHIPPER_DIR="$SCRIPT_DIR/chipper"

RPI_USER="${RPI_USER:-pi}"
RPI_IP="${RPI_IP:?Error: Set RPI_IP env var (e.g. export RPI_IP=192.168.88.100)}"

echo "=== WirePod Deploy to $RPI_USER@$RPI_IP ==="

cd "$CHIPPER_DIR"
./build-release.sh  # Build portable wire-pod binary

echo "Transferring files..."
scp wire-pod apiConfig.json customIntents.json weather-map.json wire-pod.service "$RPI_USER@$RPI_IP:/tmp/"

echo "Installing on RPi..."
ssh "$RPI_USER@$RPI_IP" '
  sudo cp /tmp/wire-pod /usr/bin/wire-pod
  sudo chmod 755 /usr/bin/wire-pod
  sudo mkdir -p /etc/wire-pod
  sudo cp /tmp/apiConfig.json /tmp/customIntents.json /tmp/weather-map.json /etc/wire-pod/
  sudo cp /tmp/wire-pod.service /etc/systemd/system/wire-pod.service
  sudo systemctl daemon-reload
  sudo systemctl enable wire-pod.service
  sudo systemctl restart wire-pod.service
  echo "Status:"
  sudo systemctl status wire-pod.service --no-pager -l
  echo "Logs (tail 20):"
  sudo journalctl -u wire-pod.service -n 20
'

echo "Deploy complete."
echo "Usage: export RPI_IP=your_rpi_ip; ./deploy-wirepod.sh"
