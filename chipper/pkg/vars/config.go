package vars

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/kercre123/wire-pod/chipper/pkg/fileutil"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
)

// a way to create a JSON configuration for wire-pod, rather than the use of env vars

var ApiConfigPath = "./apiConfig.json"

// apiConfigMu guards every read and write of APIConfig outside this
// package's own startup path (ReadConfig/CreateConfigFromEnv/WriteSTT,
// which only ever run once, before the HTTP server begins accepting
// requests -- see the comment on those functions below). GetAPIConfig,
// UpdateAPIConfig, and WriteConfigToDisk are the only sanctioned way for
// every OTHER caller to touch APIConfig: it used to be read and written
// directly from roughly 170 call sites across the backend -- dashboard
// HTTP handlers running concurrently with per-robot LLM/STT request
// goroutines, several of which even write to it mid-request (e.g. the
// Together-model default seeded by ttr.newLLMClient on first use) --
// with zero synchronization. That's a genuine, demonstrable data race
// (confirmed with go test -race, not theoretical), not a style issue.
var apiConfigMu sync.RWMutex

var APIConfig Config

// Config is exported -- unlike, say, GetBotInfo's unexported return type
// elsewhere in this package -- specifically so callers outside this
// package can write the UpdateAPIConfig(func(cfg *vars.Config) {...})
// closures the pattern below requires; an unexported parameter type
// would make that literal impossible to write from another package.
type Config struct {
	Weather struct {
		Enable   bool   `json:"enable"`
		Provider string `json:"provider"`
		Key      string `json:"key"`
		Unit     string `json:"unit"`
	} `json:"weather"`
	Knowledge struct {
		Enable                 bool    `json:"enable"`
		Provider               string  `json:"provider"`
		Key                    string  `json:"key"`
		ID                     string  `json:"id"`
		Model                  string  `json:"model"`
		IntentGraph            bool    `json:"intentgraph"`
		RobotName              string  `json:"robotName"`
		OpenAIPrompt           string  `json:"openai_prompt"`
		OpenAIVoice            string  `json:"openai_voice"`
		OpenAIVoiceWithEnglish bool    `json:"openai_voice_with_english"`
		SaveChat               bool    `json:"save_chat"`
		CommandsEnable         bool    `json:"commands_enable"`
		Endpoint               string  `json:"endpoint"`
		TopP                   float32 `json:"top_p"`
		Temperature            float32 `json:"temp"`
	} `json:"knowledge"`
	STT struct {
		Service  string `json:"provider"`
		Language string `json:"language"`
		// Whisper holds connection details for the HTTP Whisper backend,
		// which speaks the OpenAI /v1/audio/transcriptions API -- either
		// the real OpenAI API or a self-hosted compatible server (e.g.
		// faster-whisper-server). BaseURL/Model default when empty; see
		// stt/whisper.
		Whisper struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
			Model   string `json:"model"`
		} `json:"whisper"`
	} `json:"STT"`
	Server struct {
		// false for ip, true for escape pod
		EPConfig bool   `json:"epconfig"`
		Port     string `json:"port"`
		// HostOverride, when set, replaces both "escapepod.local" (EPConfig
		// mode) and the auto-detected local IP (plain IP mode) as the
		// address written into server_config.json AND as the SAN on the
		// generated TLS cert -- see botsetup.CreateServerConfig/
		// CreateCertCombo. For robots reached through a domain that isn't
		// resolvable/routable as "escapepod.local" (reverse proxies,
		// port-forwarded external domains), the endpoint the robot dials
		// and the cert's identity need to be the same value, which neither
		// existing mode can express on its own.
		HostOverride string `json:"host_override,omitempty"`
	} `json:"server"`
	HasReadFromEnv   bool `json:"hasreadfromenv"`
	PastInitialSetup bool `json:"pastinitialsetup"`
	Dashboard        struct {
		PasswordHash string `json:"password_hash,omitempty"`
		// SessionSecret backs the dashboard's session cookie. Persisting
		// it (instead of a fresh random value per process start) lets a
		// login survive a restart; see dashboardauth.getSessionToken.
		SessionSecret string `json:"session_secret,omitempty"`
	} `json:"dashboard"`
}

// writeConfig atomically persists APIConfig, logging (rather than silently
// dropping) marshal or write failures, and returns the failure too --
// callers that report success/failure back over HTTP (e.g. the dashboard
// settings endpoints) need to know this actually landed on disk rather
// than claiming success unconditionally. A write failure here (e.g. a
// permissions problem on the bind-mounted data directory) previously
// surfaced nowhere but the log, so a saved setting could silently revert
// on every restart with no indication anything had gone wrong.
//
// Callers must already hold apiConfigMu (WriteConfigToDisk/UpdateAPIConfig
// below do) or otherwise guarantee exclusive access -- the startup-only
// functions further down this file (ReadConfig/CreateConfigFromEnv/
// WriteSTT) call this directly without the lock, which is safe only
// because they run once, before the HTTP server begins accepting
// requests; see the comment on ReadConfig.
func writeConfig() error {
	writeBytes, err := json.Marshal(APIConfig)
	if err != nil {
		logger.Println("Error marshaling API config:", err)
		return err
	}
	if err := fileutil.WriteFileAtomic(ApiConfigPath, writeBytes, 0644); err != nil {
		logger.Println("Error writing API config to", ApiConfigPath, ":", err)
		return err
	}
	return nil
}

// GetAPIConfig returns a snapshot copy of the current config, safe to
// use freely without further synchronization: every field in Config is a
// plain value type -- no pointers, slices, or maps at any nesting level
// -- so the returned copy is fully independent of the live config from
// the moment this call returns. Prefer taking one snapshot at the top of
// a function over calling this repeatedly within it: a single snapshot
// also gives internally-consistent reads across multiple fields, which
// scattered direct reads of the live APIConfig never guaranteed even
// before the concurrency fix this replaced.
func GetAPIConfig() Config {
	apiConfigMu.RLock()
	defer apiConfigMu.RUnlock()
	return APIConfig
}

// UpdateAPIConfig runs fn with exclusive access to APIConfig, then
// persists the result -- the only safe way to both read current values
// and write new ones as one atomic operation (e.g. "if the provider
// changed, reset its key too"), and the only sanctioned way to mutate
// APIConfig from outside this package.
func UpdateAPIConfig(fn func(*Config)) error {
	apiConfigMu.Lock()
	defer apiConfigMu.Unlock()
	fn(&APIConfig)
	return writeConfig()
}

// WriteConfigToDisk persists the current config as-is, for the rare
// caller that needs to force a write without changing anything through
// UpdateAPIConfig (e.g. ensureCertForHostOverride, which only touches
// cert files, not APIConfig itself, but still needs its own read of
// APIConfig.Server.HostOverride and this write to happen as one
// consistent operation with respect to concurrent settings changes).
func WriteConfigToDisk() error {
	apiConfigMu.Lock()
	defer apiConfigMu.Unlock()
	logger.Println("Configuration changed, writing to disk")
	return writeConfig()
}

// SetAPIConfigInMemory mutates APIConfig under lock WITHOUT persisting --
// for the one case in this codebase that needs to change in-memory state
// for just this process's lifetime without touching the on-disk file:
// StartFromProgramInit reverting PastInitialSetup to false when a stored
// STT language turns out to be blank at startup, without overwriting a
// possibly-still-fine on-disk value it didn't itself just validate.
// Prefer UpdateAPIConfig for anything that should actually survive a
// restart -- almost everything should.
func SetAPIConfigInMemory(fn func(*Config)) {
	apiConfigMu.Lock()
	defer apiConfigMu.Unlock()
	fn(&APIConfig)
}

// CreateConfigFromEnv, WriteSTT, and ReadConfig below all access
// APIConfig directly, without apiConfigMu -- safe only because, in real
// operation, they run exactly once each, synchronously, before the HTTP
// server (config-ws, initwirepod) begins accepting any requests: see
// vars.Init, which is the only production caller of ReadConfig (which in
// turn is the only production caller of CreateConfigFromEnv/WriteSTT).
// Every caller outside this package -- where that single-threaded
// guarantee doesn't hold -- must use GetAPIConfig/UpdateAPIConfig
// instead.
func CreateConfigFromEnv() {
	// if no config exists, create it
	if os.Getenv("WEATHERAPI_ENABLED") == "true" {
		APIConfig.Weather.Enable = true
		APIConfig.Weather.Provider = os.Getenv("WEATHERAPI_PROVIDER")
		APIConfig.Weather.Key = os.Getenv("WEATHERAPI_KEY")
		APIConfig.Weather.Unit = os.Getenv("WEATHERAPI_UNIT")
	} else {
		APIConfig.Weather.Enable = false
	}
	if os.Getenv("KNOWLEDGE_ENABLED") == "true" {
		APIConfig.Knowledge.Enable = true
		APIConfig.Knowledge.Provider = os.Getenv("KNOWLEDGE_PROVIDER")
		if os.Getenv("KNOWLEDGE_PROVIDER") == "houndify" {
			APIConfig.Knowledge.ID = os.Getenv("KNOWLEDGE_ID")
		}
		APIConfig.Knowledge.Key = os.Getenv("KNOWLEDGE_KEY")
	} else {
		APIConfig.Knowledge.Enable = false
	}
	// HOST_OVERRIDE lets a deployment behind a reverse proxy/external
	// domain declare that up front, the same way every other setting
	// here seeds once from the environment -- instead of requiring a
	// visit to initial.html's connection-method form (which reconfigures
	// an already-running server) just to get the very first cert right.
	// Skipping straight to PastInitialSetup=true here means the normal
	// StartFromProgramInit flow calls StartChipper directly on this same
	// boot, with the correct cert already in place -- see
	// initwirepod.BeginWirepodSpecific, which generates that cert (this
	// package can't import the code that does, see botsetup) once it
	// observes HostOverride set with no cert on disk yet.
	if host := os.Getenv("HOST_OVERRIDE"); host != "" {
		APIConfig.Server.HostOverride = host
		APIConfig.Server.EPConfig = false
		APIConfig.Server.Port = os.Getenv("SERVER_PORT")
		if APIConfig.Server.Port == "" {
			// Matches compose.yaml's default published port -- anything
			// else requires also publishing that port there.
			APIConfig.Server.Port = "443"
		}
		APIConfig.PastInitialSetup = true
	}
	WriteSTT()
	APIConfig.HasReadFromEnv = true
	writeConfig()
}

func WriteSTT() {
	// was not part of the original code, so this is its own function
	// launched if stt not found in config
	APIConfig.STT.Service = os.Getenv("STT_SERVICE")
	if os.Getenv("STT_SERVICE") == "vosk" || os.Getenv("STT_SERVICE") == "whisper.cpp" {
		APIConfig.STT.Language = os.Getenv("STT_LANGUAGE")
	}
	// Whisper connection details are seeded independently of which
	// service is the active default, so a user can pre-configure an
	// external endpoint via compose.yaml and switch to it later from the
	// dashboard without retyping it.
	if url := os.Getenv("STT_WHISPER_URL"); url != "" {
		APIConfig.STT.Whisper.BaseURL = url
	}
	if key := os.Getenv("STT_WHISPER_KEY"); key != "" {
		APIConfig.STT.Whisper.APIKey = key
	}
	if model := os.Getenv("STT_WHISPER_MODEL"); model != "" {
		APIConfig.STT.Whisper.Model = model
	}
}

func ReadConfig() {
	// A plain os.Stat check can't tell "no config yet" apart from "a
	// zero-byte file sits at this path" -- and docker/entrypoint.sh's
	// generic persist_files()/link_file() helper deliberately creates an
	// empty placeholder file on the bind-mounted data volume for every
	// entry it manages (apiConfig.json included) whenever neither the
	// volume nor the image already has one, precisely so the destination
	// always exists for it to symlink to. On a genuinely fresh install
	// that placeholder exists before this process ever runs, so os.Stat
	// alone would report "exists" and fall into the read-existing-config
	// branch below, which fails to unmarshal zero bytes and returns
	// immediately WITHOUT ever calling CreateConfigFromEnv or writeConfig
	// -- env vars never get seeded, and the file stays empty forever,
	// since nothing else writes to it until some unrelated dashboard
	// action happens to call WriteConfigToDisk. Reproduced end-to-end
	// with the real entrypoint.sh and a real compiled binary against a
	// brand-new data directory. Treating an empty file the same as a
	// missing one -- both mean "nothing has been configured yet" -- fixes
	// this at the source instead of teaching the shell script about JSON.
	info, err := os.Stat(ApiConfigPath)
	if err != nil || info.Size() == 0 {
		CreateConfigFromEnv()
		logger.Println("API config JSON created")
	} else {
		// read config
		configBytes, err := os.ReadFile(ApiConfigPath)
		if err != nil {
			APIConfig.Knowledge.Enable = false
			APIConfig.Weather.Enable = false
			logger.Println("Failed to read API config file")
			logger.Println(err)
			return
		}
		err = json.Unmarshal(configBytes, &APIConfig)
		if err != nil {
			APIConfig.Knowledge.Enable = false
			APIConfig.Weather.Enable = false
			logger.Println("Failed to unmarshal API config JSON")
			logger.Println(err)
			return
		}
		// Env vars only ever seed the config the first time it's
		// created (CreateConfigFromEnv, above); once apiConfig.json
		// exists, the dashboard is authoritative and survives restarts
		// even if compose.yaml/the environment still says something
		// else. This used to be a special case for STT.Service that
		// resynced from the env var on every boot, silently reverting
		// dashboard changes on restart -- removed for consistency with
		// every other setting.
		if !APIConfig.HasReadFromEnv {
			if APIConfig.Server.Port != os.Getenv("DDL_RPC_PORT") {
				APIConfig.HasReadFromEnv = true
				APIConfig.PastInitialSetup = true
			}
		}

		if APIConfig.Knowledge.Model == "meta-llama/Llama-2-70b-chat-hf" {
			logger.Println("Setting Together model to Llama3")
			APIConfig.Knowledge.Model = "meta-llama/Llama-3-70b-chat-hf"
		}

		writeConfig()
		logger.Println("API config successfully read")
	}
}
