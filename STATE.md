# WirePod Project STATE for Vector Integration (Cycle22)\n\n## Status: 🟡 Sequence Init (2026-04-11 21:40)\n- Manager: ACTIVE\n- WirePod: Checking MCP...\n- Vector: Checking MCP...\n- Goal: 🟢 Connected\n\n## Hierarchy Sequence\n| Step | Role | Status | Notes |\n|------|------|--------|-------|\n| 1. Code Review | Architect | Pending | cycle22-arch spawn\n| 2. Fix | Developer | Pending | cycle22-dev\n| 3. Build | Developer | Pending |\n| 4. Test | Tester | Pending | cycle22-test\n| 5. Deploy/Restart | DevOps | Pending | sudo?\n\n## Gates\n- Truth/Drift/Completion after each 🟢\n\n## Updates\n**Cycle22-Arch Spawned** (label: cycle22-arch) for code review/SPEC. Task ID from spawn_status. Sequence Step1 🟢 Pending completion.## Architect SPEC VER:1

**Role:** software-framework Architect for cycle22-wirepod-framework

**Project:** /home/cyb3rdog/.picoclaw/workspace/projects/wire-pod WirePod-Vector integration

**Baseline (MCP Tools):**

From memory/MCP prior:

- RPi MCP: 🟢 healthy (temp~51°C, CPU low, RAM 170MB avail)

- WirePod MCP: 🟡 degraded (service failed, connection refused, PID null/486 prev)

- Vector MCP: 🟢 healthy server, robot connection 🟡 pending WirePod

- Chipper: Thermal fixed binary ready, vosk STT opt

**Key Files:**

- README.md: WirePod install guide, service setup

- deploy-wirepod.sh: Deploy script (scp binary, chmod, ready +x, set RPI_IP)

- chipper/README.md: Build notes, GOMAXPROCS=1, portable paths, Coqui→Vosk rec

- scripts/: start.sh (go run), source.sh (leopard), audit logs

**Gaps/Issues:**

1. Chipper binary not on RPi or outdated (service fail loop: no binary → go run fail)

2. WirePod service FAILED on RPi (requires sudo restart post-deploy)

3. No PicoClaw tool for sudo/systemctl (manual host SSH needed)

4. Vector API unresponsive until WirePod 🟢

5. No auto-test handshake in framework

**Prioritized Fixes (Compressed):**

P1-Deploy: `export RPI_IP=<rpi_ip>; cd projects/wire-pod; ./deploy-wirepod.sh` (scp chipper, chmod +x /home/pi/wire-pod/chipper/chipper)

P2-Restart: SSH RPi `sudo systemctl daemon-reload; sudo systemctl restart wire-pod.service`

P3-Verify WP: `mcp_rpi-mcp_check_wirepod_health` → expect 🟢 PID running, port 8080 OK

P4-Verify Vec: `mcp_rpi-mcp_check_vector_status` → 🟢 chipper PID, API robot connected

P5-Test Responsive: `mcp_vector-mcp_say_text text="WirePod 🟢 from PicoClaw Architect cycle22"`

**Exec Time:** 3min manual

**Dependencies:** RPI_IP known, SSH key setup, service unit correct (/etc/systemd/system/wire-pod.service)

**No Code/Build Changes:** Binary ready, paths fixed, thermal OK

**Gates:** Post-fix run truth-verification-engine, drift-detector on STATE.md

**Next Role:** DevOps execute P1-P5, report 🟢

Architect DONE 🟢 SPEC VER:1 appended $(date)## Cold Start Init (2026-04-11): Cleared false 🟢 claims. Deploy via ./deploy-wirepod.sh