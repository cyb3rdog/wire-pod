package vars

import (
	"encoding/json"
	"os"

	"github.com/kercre123/wire-pod/chipper/pkg/fileutil"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
)

// a way to create a JSON configuration for wire-pod, rather than the use of env vars

var ApiConfigPath = "./apiConfig.json"

var APIConfig apiConfig

type apiConfig struct {
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
	// Advanced holds deployment-behavior toggles that used to be
	// env-var-only with no dashboard page and no persistence at all --
	// user-scope preferences about *this* instance (thermal tuning, which
	// listeners to bind, mDNS, bot remote control), not system-scope
	// infrastructure config, so they belong here like everything else the
	// dashboard manages. AdvancedMigrated marks a config that's already
	// gone through the one-time migration (see migrateAdvancedSettings)
	// that seeds these from whatever the old env vars said, so it only
	// ever runs once per install -- these are dashboard-authoritative
	// from that point on, same as every other setting in this struct.
	Advanced struct {
		VoskThermalEnabled bool `json:"vosk_thermal_enabled"`
		VoskWithGrammar    bool `json:"vosk_with_grammar"`
		DisableMDNS        bool `json:"disable_mdns"`
		Port8084Enabled    bool `json:"port8084_enabled"`
		JdocsPingerEnabled bool `json:"jdocs_pinger_enabled"`
		SDKEnabled         bool `json:"sdk_enabled"`
		Port80Enabled      bool `json:"port80_enabled"`
	} `json:"advanced"`
	AdvancedMigrated bool `json:"advanced_migrated"`
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

func WriteConfigToDisk() error {
	logger.Println("Configuration changed, writing to disk")
	return writeConfig()
}

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
	migrateAdvancedSettings()
	APIConfig.HasReadFromEnv = true
	writeConfig()
}

// migrateAdvancedSettings seeds APIConfig.Advanced from whatever the old
// env-var-only toggles (VOSK_THERMAL_ENABLED, VOSK_WITH_GRAMMER,
// DISABLE_MDNS, NO8084, JDOCS_PINGER_ENABLED, SDK_ENABLED, PORT80_ENABLED)
// say right now, for continuity with existing compose.yaml deployments --
// but only once per install, guarded by AdvancedMigrated, matching how
// every other setting in this struct only ever seeds from the
// environment on a genuinely fresh config. After this runs, the
// dashboard is authoritative for these; the env vars are never consulted
// again.
func migrateAdvancedSettings() {
	if APIConfig.AdvancedMigrated {
		return
	}
	APIConfig.Advanced.VoskThermalEnabled = os.Getenv("VOSK_THERMAL_ENABLED") != "false"
	APIConfig.Advanced.VoskWithGrammar = os.Getenv("VOSK_WITH_GRAMMER") == "true"
	APIConfig.Advanced.DisableMDNS = os.Getenv("DISABLE_MDNS") == "true"
	// NO8084 is inverted-sense (true means disabled) and predates the
	// _ENABLED convention used everywhere else here.
	APIConfig.Advanced.Port8084Enabled = os.Getenv("NO8084") != "true"
	APIConfig.Advanced.JdocsPingerEnabled = os.Getenv("JDOCS_PINGER_ENABLED") != "false"
	APIConfig.Advanced.SDKEnabled = os.Getenv("SDK_ENABLED") != "false"
	APIConfig.Advanced.Port80Enabled = os.Getenv("PORT80_ENABLED") != "false"
	APIConfig.AdvancedMigrated = true
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
	if _, err := os.Stat(ApiConfigPath); err != nil {
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

		migrateAdvancedSettings()

		writeConfig()
		logger.Println("API config successfully read")
	}
}
