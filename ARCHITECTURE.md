# wire-pod Architecture Reference

> **Master knowledge file** - Contains everything needed to understand, modify, and debug wire-pod without re-researching.

## Last Updated: 2026-04-08

---

## Project Overview

wire-pod is a **opensource implementation** for Anki Vector robot Chipper service. It provides:
- HTTP REST API + WebSocket for robot control
- Custom wake word detection (VOSK/Leopard/Coqui)
- Speech-to-text (STT) via chipper service
- Cloud gateway integration (vector-cloud)
- Plugin system for extensibility

---

## Directory Structure

```
wire-pod/
├── chipper/           # Main STT + Intent processing service (Go)
│   ├── cmd/           # Entry points (chipper, vosk, leopard, coqui)
│   ├── pkg/
│   │   ├── initwirepod/   # Initialization logic
│   │   ├── logger/        # Logging utilities
│   │   ├── mdnshandler/   # mDNS discovery (Vector pairing)
│   │   ├── scripting/     # Scripting engine (Lua/exec plugins)
│   │   ├── servers/       # HTTP/WebSocket servers
│   │   ├── vars/          # Global variables (thread-safe with mutexes)
│   │   ├── vtt/           # Voice Text Terminal
│   │   ├── wirepod/      # Core intent processing
│   │   │   ├── stt/       # Speech-to-Text adapters
│   │   │   │   ├── vosk/   # VOSK adapter ⚠️ HIGH CPU (thermal fix applied)
│   │   │   │   ├── leopard/  # Leopard STT adapter
│   │   │   │   ├── coqui/    # Coqui STT adapter
│   │   │   │   ├── whisper/  # Whisper STT adapter
│   │   │   │   └── houndify/ # Houndify STT adapter
│   │   │   ├── ttr/       # Text-to-Response processing
│   │   │   ├── sdkapp/    # SDK application handlers
│   │   │   ├── preqs/     # Processing requests
│   │   │   ├── setup/     # Setup utilities
│   │   │   └── config-ws/ # WebSocket config
│   │   └── ...            # Other packages
│   ├── jdocs/            # Robot document storage
│   ├── intent-data/      # Intent definitions (15 languages)
│   ├── webroot/          # (Embedded in binary or served separately)
│   ├── plugins/          # Plugin executables
│   └── go.mod            # Go dependencies
│
├── vector-cloud/       # Cloud gateway service (Go)
│   ├── cmd/            # Entry points
│   ├── internal/       # Internal packages
│   ├── gateway/        # Gateway logic
│   └── cloud/          # Cloud communication
│
├── vosk/               # VOSK model files storage
├── certs/              # TLS certificates
├── setup.sh            # Installation script
├── update.sh           # Update script
└── dockerfile          # Docker build config
```

---

## Architecture Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         Vector Robot                             │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐                   │
│  │ wakeword │───▶│  audio   │───▶│  intent  │                   │
│  │ detector │    │ stream   │    │ resolver │                   │
│  └──────────┘    └──────────┘    └──────────┘                   │
└─────────────────────────────────────────────────────────────────┘
         │                 │                  │
         ▼                 ▼                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                    wire-pod Services                             │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐       │
│  │   chipper    │───▶│ vector-cloud │───▶│    HTTP/WS   │       │
│  │ (STT+Intent) │    │  (gateway)   │    │    API       │       │
│  └──────────────┘    └──────────────┘    └──────────────┘       │
│         │                                       │                │
│         ▼                                       ▼                │
│  ┌──────────────┐                      ┌──────────────┐         │
│  │ VOSK/Leopard │                      │  Clients     │         │
│  │   (STT)      │                      │ (vector-mcp) │         │
│  └──────────────┘                      └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

---

## Key Components

### 1. chipper (Main Service)

**Purpose**: Speech recognition + intent processing

**Entry Point**: `chipper/cmd/` directories contain main.go variants

**Core Packages**:
| Package | Purpose | Notes |
|---------|---------|-------|
| `vosk/` | VOSK speech recognition |  |
| `leopard/` | Leopard speech recognition | Alternative STT |
| `coqui/` | Coqui STT | Alternative STT |
| `wirepod/` | Intent processing | Core logic |
| `epod/` | Extended intent handler | Experimental |
| `servers/` | HTTP/WebSocket servers | API endpoints |
| `mdnshandler/` | mDNS for Vector discovery | Pairing mechanism |

### 2. vector-cloud (Gateway Service)

**Purpose**: Bridge between chipper and Vector cloud infrastructure, runs on the Vector robot, not locally.

**Key Files**:
- `gateway/` - Main gateway logic
- `cloud/` - Cloud protocol implementation

### 3. STT Engines

| Engine | Package | Status | Performance |
|--------|---------|--------|-------------|
| VOSK | `chipper/pkg/vosk/` | Default | ⚠️ High CPU (~45% on RPi Zero 2W) |
| Leopard | `chipper/pkg/leopard/` | Alternative | Better performance expected |
| Coqui | `chipper/pkg/coqui/` | Alternative | GPU-friendly |

---

## Global State Management (`chipper/pkg/vars/`)

> **Analysis Date**: 2026-04-09 | **Status**: In Progress

### Overview

The `vars` package contains global variables that manage:
- Configuration state (API config, STT settings)
- Robot registry (Vector bot information)
- Session management (TLS certificates)
- Custom intents and intent data
- File paths and runtime directories

**Files**:
- `vars.go` - Main global variables and initialization
- `config.go` - Configuration management

---

### Key Global Variables

#### Configuration Variables (config.go)

| Variable | Type | Purpose |
|----------|------|---------|
| `APIConfig` | `apiConfig` | Main configuration struct (weather, knowledge, STT, server) |
| `ApiConfigPath` | `string` | Path to apiConfig.json (default: `./apiConfig.json`) |

#### Bot Registry State

| Variable | Type | Purpose |
|----------|------|---------|
| `BotInfo` | `RobotInfoStore` | Registered Vector robots (ESN, IP, GUID, activated status) |
| `BotJdocs` | `[]botjdoc` | Robot document storage (thing, name, jdoc) |
| `RecurringInfo` | `[]RecurringInfoStore` | Session info for connected robots (ESN, ID, IP) |

**Structs**:
```go
type RobotInfoStore struct {
    GlobalGUID string `json:"global_guid"`
    Robots     []struct {
        Esn       string `json:"esn"`
        IPAddress string `json:"ip_address"`
        GUID      string `json:"guid"`
        Activated bool   `json:"activated"`
    } `json:"robots"`
}

type RecurringInfoStore struct {
    ID   string `json:"id"`   // Certificate CN
    ESN  string `json:"esn"`
    IP   string `json:"ip"`
}
```

#### Custom Intents

| Variable | Type | Purpose |
|----------|------|---------|
| `CustomIntents` | `[]CustomIntent` | Loaded custom intents from customIntents.json |
| `CustomIntentsExist` | `bool` | Flag indicating if custom intents are loaded |
| `IntentList` | `[]JsonIntent` | Parsed intent definitions |

#### STT/TTR State

| Variable | Type | Purpose |
|----------|------|---------|
| `DownloadedVoskModels` | `[]string` | Available VOSK models in vosk/models/ |
| `VoskGrammerEnable` | `bool` | Grammar enabled flag |
| `SttInitFunc` | `func() error` | STT initialization function (prevents import cycle) |

#### File Paths & Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `JdocsPath` | `./jdocs/jdocs.json` | Robot documents storage |
| `JdocsDir` | `./jdocs` | Jdocs directory |
| `CustomIntentsPath` | `./customIntents.json` | Custom intents file |
| `BotConfigsPath` | `./botConfig.json` | Bot configurations |
| `BotInfoPath` | `./jdocs/botSdkInfo.json` | Bot SDK info |
| `VoskModelPath` | `../vosk/models/` | VOSK model directory |
| `WhisperModelPath` | `../whisper.cpp/models/` | Whisper model directory |
| `SessionCertPath` | `./session-certs/` | TLS certificates for robot sessions |
| `SDKIniPath` | `/.anki_vector/` | SDK configuration path |
| `WebPort` | `8080` | HTTP server port |
| `PodName` | `wire-pod` | Application name |

#### TLS/Certificate State

| Variable | Type | Purpose |
|----------|------|---------|
| `ChipperCert` | `[]byte` | TLS certificate bytes |
| `ChipperKey` | `[]byte` | TLS key bytes |
| `ChipperKeysLoaded` | `bool` | Flag if certificates loaded |

#### Runtime Flags

| Variable | Type | Purpose |
|----------|------|---------|
| `VarsInited` | `bool` | Initialization flag (prevent re-init) |
| `Packaged` | `bool` | Packaged installation flag |
| `IsPackagedLinux` | `bool` | Linux packaging flag |
| `AndroidPath` | `string` | Path on Android |
| `CommitSHA` | `string` | Git commit of build |

---

### Initialization Flow

```
Init() (vars.go:122)
├── ReadConfig() → loads apiConfig.json
├── GetDownloadedVoskModels() → scans vosk/models/
├── Load BotJdocs from JdocsPath
├── Load BotInfo from BotInfoPath
├── ReadSessionCerts() → loads TLS certs from SessionCertPath
└── LoadCustomIntents() → loads customIntents.json
```

---

### Thread Safety (Updated 2026-04-09)

✅ **FIXED**: All identified race conditions now have mutex protection.

| Global Variable | Protection | Status |
|-----------------|------------|--------|
| `BotJdocs` | `botJdocsMu sync.RWMutex` | ✅ Protected |
| `RecurringInfo` | `recurringInfoMu sync.RWMutex` | ✅ Protected |
| `RememberedChats` | `rememberedChatsMu sync.RWMutex` | ✅ Protected |
| `getRec()` pool | `recsmu sync.Mutex` + size limit | ✅ Protected |

**VOSK Thermal Management**:
- Idle detection with 30s timeout
- Temperature monitoring via `/sys/class/thermal/`
- Adaptive throttling (500ms-2000ms delay)
- Recognition pool limited to 5 per pool

**Remaining Concerns**:
- `stopCh` channel in Vosk.go idle monitor (potential goroutine leak)
- `GetRobot()` reads `BotInfo.Robots` without explicit lock
- `DownloadedVoskModels` modified without lock

**Recommended**:
1. Add shutdown handler to close idle monitor goroutine
2. Add `sync.RWMutex` to `BotInfo` access

---

## Configuration Files

| File | Purpose |
|------|---------|
| `chipper/apiConfig.json` | API configuration, intent settings |
| `chipper/customIntents.json` | Custom intent definitions |
| `chipper/weather-map.json` | Weather location mappings |
| `chipper/source.sh` | Environment setup |
| `chipper/start.sh` | Service startup script |

---

## Critical Paths for Review

### High CPU Usage Investigation
1. `chipper/pkg/vosk/` - VOSK adapter
2. `chipper/pkg/servers/` - Server request handling
3. Main loop in `chipper/cmd/` entry points

### Race Condition Hotspots
1. `chipper/pkg/vars/` - Global variable access
2. `chipper/pkg/servers/` - Concurrent connection handling
3. `chipper/pkg/mdnshandler/` - Discovery state management

---

## Dependencies

**chipper/go.mod** - Key dependencies:
- `github.com/upper/db/v4` - Database
- `github.com/gorilla/websocket` - WebSocket
- `github.com/asticode/go-astikit` - Utilities

---

## Setup & Running

```bash
# Setup
./setup.sh

# Start chipper
cd chipper && ./start.sh

# Start vector-cloud (separate process)
cd vector-cloud && ./start.sh
```

---

## Integration Points

### For vector-mcp Project
- HTTP API endpoints on configurable port (default: 8080)
- WebSocket for real-time events
- Can serve as alternative/failover to vector_mcp_server.py

---

## TODO: Add More Sections
- [ ] API endpoint documentation
- [ ] Intent processing flow details
- [ ] Plugin system documentation
- [ ] Testing strategy
- [ ] Performance benchmarks

---

## Global State Management (`chipper/pkg/vars/`)

### Files Analyzed
- `vars.go` (13,445 bytes) - Core variables and functions
- `config.go` (4,254 bytes) - Configuration management

### Key Global Variables

| Variable | Type | Purpose |
|----------|------|---------|
| `APIConfig` | `apiConfig` struct | Main configuration (weather, knowledge, STT, server) |
| `BotInfo` | `RobotInfoStore` | Registered robots (ESN, IP, GUID, activated status) |
| `BotJdocs` | `[]botjdoc` | Robot document storage |
| `CustomIntents` | `[]CustomIntent` | Custom voice intents |
| `CustomIntentsExist` | bool | Flag for intent loading |
| `RecurringInfo` | `[]RecurringInfoStore` | Session certificates per robot |
| `RememberedChats` | `[]RememberedChat` | LLM conversation history |
| `VoskModelPath` | string | Path to VOSK models (`../vosk/models/`) |
| `DownloadedVoskModels` | []string | Available VOSK language models |
| `ChipperCert/Key` | []byte | TLS certificate/key for chipper |
| `WebPort` | string | Web server port (default 8080) |
| `SDKIniPath` | string | Path to `.anki_vector/` SDK config |
| `VarsInited` | bool | Initialization flag to prevent re-init |

### Configuration Structure (apiConfig)

```go
type apiConfig struct {
    Weather      // weather API settings
    Knowledge   // LLM provider, model, prompt
    STT         // Speech-to-text: provider, language
    Server      // Port, EPConfig
}
```

### Concurrency Considerations

1. **Mutex-protected recognizer pool** in VOSK adapter (`recsmu`)
2. **Global APIConfig** - Read during requests, Write via web UI
3. **BotJdocs** - No explicit mutex, potential race on concurrent writes
4. **RecurringInfo** - Append-only during init, read-only after

### Thread Safety Notes

- `vars.Init()` runs once with `VarsInited` guard
- Config writes use `WriteConfigToDisk()` with JSON marshal
- No runtime mutex protection on shared state (potential issue)

---

*Updated: 2026-04-09 - Added vars package analysis*
