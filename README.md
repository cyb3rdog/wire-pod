# wire-pod

wire-pod is a self-hosted reimplementation of Anki's "Chipper" cloud service for the Anki Vector robot. Anki's own cloud service for Vector was shut down; wire-pod lets a Vector robot keep working by talking to a server you run yourself instead, over the same gRPC protocol the robot's firmware already speaks.

## What it does

- Speech-to-text for Vector's voice commands, via a pluggable backend: [Vosk](https://alphacephei.com/vosk/) (offline, default), [Picovoice Leopard](https://picovoice.ai/platform/leopard/), OpenAI's Whisper API, or a local [whisper.cpp](https://github.com/ggerganov/whisper.cpp) build.
- Built-in intent matching for Vector's stock voice commands (time, weather, jokes, movement, etc.), plus **custom intents** you define yourself: match a phrase to a shell command, a Lua script, or an existing robot intent.
- Optional LLM/knowledge-graph fallback: when nothing matches, forward the transcribed speech to an OpenAI-compatible API and have Vector speak the response.
- A Go plugin system (`chipper/plugins/`) for hooking new voice commands into the pipeline with compiled `.so` plugins.
- A local web UI (default `:8080`) for pairing robots, picking an STT backend/language, and managing custom intents, plugins, and API keys.

## How it fits together

wire-pod runs the same jdocs/token/chipper gRPC services Vector's firmware expects, on the same ports Anki's cloud used to serve. Once a robot is pointed at your server (via a BLE pairing flow or an "escape pod" cert), it sends audio for STT, wire-pod matches the transcribed text against intents, and a response streams back to the robot the same way it always did.

## Setup

### Docker (recommended)

```sh
docker compose up -d --build
```

Run from the repository root. This builds and runs the image described in `dockerfile`/`compose.yaml`, persisting config, certs, and jdocs under a named volume. See `docker/entrypoint.sh` for the environment variables it honors (`WIREPOD_STT_SERVICE`, `WIREPOD_STT_LANGUAGE`, `WIREPOD_DEBUG_LOGGING`, etc).

### Native (Linux/macOS)

```sh
sudo ./setup.sh      # installs build deps, fetches STT assets, generates certs
sudo ./chipper/start.sh
```

`setup.sh` supports Debian/apt, Arch/pacman, Fedora/dnf, and macOS (via Homebrew). Run `sudo ./setup.sh daemon-enable` afterward to install it as a systemd service (see `chipper/wire-pod.service`).

### After starting

Open the web UI at `http://<host>:8080` to pair your Vector robot and pick an STT engine/language (BLE setup or escape-pod mode, depending on your robot's firmware).

## Security note

The web UI (`:8080`) and the Lua-scripting endpoint (`:80`) are **not authenticated** — they're meant to be reached only from a trusted local network. If you expose wire-pod beyond your LAN, put it behind your own reverse proxy with auth (e.g. Caddy with `basicauth`, or an authenticating proxy in front of it) rather than exposing those ports directly.

## Repository layout

- `chipper/` — the actual server: `cmd/` has one entrypoint per STT backend, `pkg/` has the gRPC services, intent matching, STT backends, and web UI.
- `vector-cloud/` — Anki's original on-robot cloud-process source, kept for reference.
- `setup.sh` / `update.sh` — native install/update scripts.
- `dockerfile` / `compose.yaml` / `docker/` — container build and entrypoint.
