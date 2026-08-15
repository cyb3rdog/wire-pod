// Package dispatch routes STT requests to whichever backend is currently
// selected in vars.APIConfig.STT.Service, so cmd/vosk's single compiled
// binary (the one the Docker image builds) can serve either the local
// Vosk engine or an HTTP Whisper endpoint -- the real OpenAI API, or a
// self-hosted compatible server such as faster-whisper-server -- and
// switch between them live from the dashboard, with no rebuild or
// restart.
//
// Both backends already share the Init()/STT() signature preqs.New()
// expects, and neither needs a native dependency beyond what cmd/vosk
// already links (Whisper is pure Go), so wiring both into the same
// binary costs nothing at build time.
package dispatch

import (
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	vosk "github.com/kercre123/wire-pod/chipper/pkg/wirepod/stt/vosk"
	whisper "github.com/kercre123/wire-pod/chipper/pkg/wirepod/stt/whisper"
)

// Name is passed to preqs.New() as the initial voiceProcessor label. It's
// intentionally static ("vosk") rather than read from vars.APIConfig: the
// caller (cmd/vosk/main.go) has to pass it before vars.Init() has loaded
// the persisted config, so there's no live value available yet at that
// point. The one thing New() does with it (besides an informational log
// line) is skip force-resetting STT.Language to en-US for the vosk/
// whisper.cpp backends -- "vosk" is the safe choice there since it's also
// the default STT_SERVICE, and getting this wrong for a returning
// whisper-selected install would only misdescribe one log line, not
// affect behavior.
var Name = "vosk"

func useWhisper() bool {
	return vars.GetAPIConfig().STT.Service == whisper.Name
}

// Init initializes only whichever backend is actually selected -- vosk's
// Init loads a model file from disk, which is both wasted work and would
// fail outright (no model downloaded) when Whisper is the active service.
func Init() error {
	if useWhisper() {
		return whisper.Init()
	}
	return vosk.Init()
}

func STT(req sr.SpeechRequest) (string, error) {
	if useWhisper() {
		return whisper.STT(req)
	}
	return vosk.STT(req)
}
