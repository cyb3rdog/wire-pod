package processreqs

import (
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	ttr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/ttr"
)

// Server stores the config
type Server struct{}

var VoiceProcessor = ""

type JsonIntent struct {
	Name              string   `json:"name"`
	Keyphrases        []string `json:"keyphrases"`
	RequireExactMatch bool     `json:"requiresexact"`
}

var sttLanguage string = "en-US"

// speech-to-text
var sttHandler func(sr.SpeechRequest) (string, error)

// ReloadVosk re-runs the compiled binary's STT init function after a
// settings change (language, or -- via cmd/vosk's dispatcher -- the
// active service itself). Previously gated to "vosk"/"whisper.cpp" only;
// unconditional now that cmd/vosk's Init can itself be a dispatcher
// routing to whichever backend is actually selected, and every other
// backend's Init is a cheap, idempotent no-op safe to re-run.
func ReloadVosk() {
	vars.SttInitFunc()
	vars.IntentList, _ = vars.LoadIntents()
}

// New returns a new server
func New(InitFunc func() error, SttHandler func(sr.SpeechRequest) (string, error), voiceProcessor string) (*Server, error) {

	// Decide the TTS language -- in-memory only (no persist call, same as
	// the direct field assignment this replaces): this backend doesn't
	// have its own language setting, "en-US" is just what this process
	// uses internally, not a saved preference.
	if voiceProcessor != "vosk" && voiceProcessor != "whisper.cpp" {
		vars.SetAPIConfigInMemory(func(cfg *vars.Config) { cfg.STT.Language = "en-US" })
	}
	sttLanguage = vars.GetAPIConfig().STT.Language
	vars.IntentList, _ = vars.LoadIntents()
	logger.Println("Initiating " + voiceProcessor + " voice processor with language " + sttLanguage)
	vars.SttInitFunc = InitFunc
	err := InitFunc()
	if err != nil {
		return nil, err
	}

	sttHandler = SttHandler

	// Initiating the chosen voice processor and load intents from json
	VoiceProcessor = voiceProcessor

	// Load plugins
	ttr.LoadPlugins()

	return &Server{}, err
}
