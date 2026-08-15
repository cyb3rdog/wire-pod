# wire-pod

wire-pod is a self-hosted reimplementation of Anki's "Chipper" cloud service for the Anki Vector robot. Anki's own cloud service for Vector was shut down; wire-pod lets a Vector robot keep working by talking to a server you run yourself instead, over the same gRPC protocol the robot's firmware already speaks.

## What it does

- Speech-to-text for Vector's voice commands, via a pluggable backend: [Vosk](https://alphacephei.com/vosk/) (offline, default), [Picovoice Leopard](https://picovoice.ai/platform/leopard/), a local [whisper.cpp](https://github.com/ggerganov/whisper.cpp) build, or an HTTP Whisper endpoint -- the real OpenAI API, or a self-hosted OpenAI-compatible server such as `faster-whisper-server`. The Docker image builds Vosk and the HTTP Whisper backend into the same binary and lets you switch between them live from the dashboard's **Server Settings → STT Service** page, no rebuild or restart required.
- Built-in intent matching for Vector's stock voice commands (time, weather, jokes, movement, etc.), plus **custom intents** you define yourself: match a phrase to a shell command, a Lua script, or an existing robot intent.
- Optional LLM/knowledge-graph fallback: when nothing matches, forward the transcribed speech to an OpenAI-compatible API and have Vector speak the response.
- A Go plugin system (`chipper/plugins/`) for hooking new voice commands into the pipeline with compiled `.so` plugins. Go's `plugin` package requires the plugin to be built with the exact same Go toolchain version and dependency versions as the wire-pod binary loading it — a mismatch fails to load at runtime rather than at compile time, so build plugins against the same `go.mod`/`go.sum` and Go version wire-pod itself uses, and rebuild them whenever you update wire-pod.
- A local web UI (default `:8080`) for pairing robots, picking an STT backend/language, and managing custom intents, plugins, and API keys.
- An optional SDK app server (port `80`: bot remote-control features like eye color/volume/camera streaming, plus the jdocs pinger) that wire-pod uses as an SDK *client* to reach back into the robot -- architecturally separate from the core voice pipeline above, which never depends on it. Set `SDK_ENABLED=false` (`WIREPOD_SDK_ENABLED` in Docker) to turn all of that off and hide the dashboard's now-nonfunctional "Bot Settings" page. This is decoupled from whether port 80 itself is bound: `/ok` (and, if `SDK_ENABLED` is on, the SDK endpoints too) is registered on the same handler table `:8080` serves, so it stays reachable there regardless. Set `PORT80_ENABLED=false` (`WIREPOD_PORT80_ENABLED` in Docker) if you don't want a separate port 80 socket at all -- e.g. only forwarding `:8080` through a reverse proxy -- keeping in mind a paired robot's own connectivity check-in only ever asks for port 80 (`server_config.json`'s `"check"` field has no way to point it elsewhere) and won't get answered once this is off.

## How it fits together

wire-pod runs the same jdocs/token/chipper gRPC services Vector's firmware expects, on the same ports Anki's cloud used to serve. Once a robot is pointed at your server (via a BLE pairing flow or an "escape pod" cert), it sends audio for STT, wire-pod matches the transcribed text against intents, and a response streams back to the robot the same way it always did.

## Setup

### Docker (recommended)

```sh
docker compose up -d --build
```

Run from the repository root. This builds and runs the image described in `dockerfile`/`compose.yaml`, persisting config, certs, jdocs, and downloaded models under `./data` and generated images under `./images` -- plain directories next to the compose file, not Docker-managed volumes, so they're easy to inspect or back up directly. (The container starts as root just long enough to fix their ownership if needed, then drops to an unprivileged user -- see `docker/entrypoint.sh`.)

`compose.yaml` declares every setting it supports as an environment variable with a default, so you can override any of them via a `.env` file next to `compose.yaml` or `WIREPOD_FOO=bar docker compose up -d`, without editing the file itself. Most of these (STT service/language/Whisper endpoint, knowledge-graph "Ask" toggle, weather, debug logging, BLE) have a dashboard equivalent and **only seed the very first boot** (before `./data/chipper/apiConfig.json` exists) -- once wire-pod is running, change them from the dashboard's Server Settings page instead; it's authoritative from then on and survives restarts. A few (Vosk's thermal management/grammar tuning, mDNS, the legacy `:8084` listener, the jdocs pinger, the SDK app server) have no dashboard page at all and apply live on every restart instead. See `compose.yaml` for the full list and which category each one falls into.

### Native (Linux/macOS)

```sh
sudo ./setup.sh      # installs build deps, fetches STT assets, generates certs
sudo ./chipper/start.sh
```

`setup.sh` supports Debian/apt, Arch/pacman, Fedora/dnf, and macOS (via Homebrew). Run `sudo ./setup.sh daemon-enable` afterward to install it as a systemd service (see `chipper/wire-pod.service`).

### After starting

Open the web UI at `http://<host>:8080` to pair your Vector robot and pick an STT engine/language (BLE setup or escape-pod mode, depending on your robot's firmware).

### Reaching wire-pod through a custom domain (reverse proxy / external access)

wire-pod's self-signed cert (`certs/cert.crt`) only ever carries one identity: whatever address it's told the robot will actually dial, either as an `IPAddresses` SAN (auto-detected LAN IP) or a `DNSNames` SAN (a domain). "Escape Pod" mode hardcodes `escapepod.local`, which only resolves over local mDNS and was never in the cert's SAN at all -- if the robot isn't on the same LAN segment as wire-pod (e.g. reached through a domain forwarded via Caddy/nginx), what it dials and what the cert claims to be need to be the same value, or the robot's TLS validation rejects the connection ("certificate is valid for escapepod.local, not your-domain").

**Recommended: declare it before first boot**, the same way STT/Knowledge/Weather are seeded -- set in `compose.yaml`/`.env`:

```
WIREPOD_HOST_OVERRIDE=wirepod.example.com
WIREPOD_SERVER_PORT=443   # optional, defaults to 443 (already published below)
```

then start (or restart, on a genuinely fresh `./data`) the container. The correct cert gets generated before the gRPC server ever binds -- no need to touch `initial.html` at all. Like every other setting in this list, this only ever seeds a *fresh* setup: once a cert exists on disk, it's left alone on every later restart, whether that's still this same value or something changed afterward through the dashboard.

If you set `WIREPOD_SERVER_PORT` to anything other than 443, you also need to add a matching entry to `compose.yaml`'s `ports:` section yourself -- Docker doesn't publish a port nothing declares, and wire-pod binding it internally doesn't make it reachable on its own.

**Alternative: change it after the fact**, without wiping `./data` -- the initial setup page's "Connection Method" has a third option, **Custom Host**, that does the same regeneration (cert + `certs/server_config.json`) against an already-configured server. Since `initial.html`'s form is otherwise a first-run-only page, reaching it again on an already-set-up server requires logging in first (it'll redirect you to `/login.html` automatically if needed).

Either way, deploying the robot side needs **two** files, not just `server_config.json`: also copy the regenerated `certs/cert.crt` onto the robot (the dashboard's "Bot Setup" SSH page does this automatically for dev-unlocked robots; see `chipper/pkg/wirepod/setup/ssh.go` for the manual file paths otherwise) -- `server_config.json` alone only tells the robot where to connect, not what to trust when it gets there.

## Security note

The web UI (`:8080`) and the Lua-scripting/session-cert endpoints (`:80`) are gated by a password you set on first visit (see `chipper/pkg/wirepod/dashboardauth/`); the initial robot-pairing pages stay reachable before that password exists, since nothing can be logged into yet. The login session is persisted, so a restart doesn't sign you back out -- it rotates (invalidating any existing session) only when the password itself changes. This is meant as a baseline, not a substitute for network isolation — if you expose wire-pod beyond your LAN, still put it behind your own reverse proxy with auth (e.g. Caddy with `basicauth`, or an authenticating proxy in front of it) rather than relying on it alone.

## Repository layout

- `chipper/` — the actual server: `cmd/` has one entrypoint per STT backend, `pkg/` has the gRPC services, intent matching, STT backends, and web UI.
- `vector-cloud/` — Anki's original on-robot cloud-process source, kept for reference.
- `setup.sh` / `update.sh` — native install/update scripts.
- `dockerfile` / `compose.yaml` / `docker/` — container build and entrypoint.
