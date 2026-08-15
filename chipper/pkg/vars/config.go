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
// dropping) marshal or write failures.
func writeConfig() {
	writeBytes, err := json.Marshal(APIConfig)
	if err != nil {
		logger.Println("Error marshaling API config:", err)
		return
	}
	if err := fileutil.WriteFileAtomic(ApiConfigPath, writeBytes, 0644); err != nil {
		logger.Println("Error writing API config to", ApiConfigPath, ":", err)
	}
}

func WriteConfigToDisk() {
	logger.Println("Configuration changed, writing to disk")
	writeConfig()
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

		writeConfig()
		logger.Println("API config successfully read")
	}
}
