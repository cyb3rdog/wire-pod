#!/bin/bash
# RPi WirePod Deploy Protocol - Host Execution (sets RPI_IP env or fail)

set -euo pipefail

SCRIPT_DIR=&quot;$(cd &quot;$(dirname &quot;${BASH_SOURCE[0]}&quot;)&quot; &amp;&amp; pwd)&quot;
CHIPPER_DIR=&quot;$SCRIPT_DIR/chipper&quot;

RPI_USER=&quot;${RPI_USER:-pi}&quot;
RPI_IP=&quot;${RPI_IP:?Error: Set RPI_IP env var (e.g. export RPI_IP=192.168.88.100)}&quot;

echo &quot;=== WirePod Deploy to $RPI_USER@$RPI_IP ===&quot;

cd &quot;$CHIPPER_DIR&quot;
./build-release.sh  # Build portable wire-pod binary

echo &quot;Transferring files...&quot;
scp wire-pod apiConfig.json customIntents.json weather-map.json wire-pod.service &quot;$RPI_USER@$RPI_IP:/tmp/&quot;

echo &quot;Installing on RPi...&quot;
ssh &quot;$RPI_USER@$RPI_IP&quot; $&apos;
  sudo cp /tmp/wire-pod /usr/bin/wire-pod
  sudo chmod 755 /usr/bin/wire-pod
  sudo mkdir -p /etc/wire-pod
  sudo cp /tmp/apiConfig.json /tmp/customIntents.json /tmp/weather-map.json /etc/wire-pod/
  sudo cp /tmp/wire-pod.service /etc/systemd/system/wire-pod.service
  sudo systemctl daemon-reload
  sudo systemctl enable wire-pod.service
  sudo systemctl restart wire-pod.service
  echo &quot;Status:&quot;
  sudo systemctl status wire-pod.service --no-pager -l
  echo &quot;Logs (tail 20):&quot;
  sudo journalctl -u wire-pod.service -n 20
&apos;

echo &quot;✅ Deploy COMPLETE. Check WirePod MCP health.&quot;
echo &quot;Usage: export RPI_IP=your_rpi_ip; ./deploy-wirepod.sh&quot;