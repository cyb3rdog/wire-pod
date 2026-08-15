package dispatch

import (
	"testing"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

func TestUseWhisperRoutesOnConfiguredService(t *testing.T) {
	orig := vars.APIConfig.STT.Service
	t.Cleanup(func() { vars.APIConfig.STT.Service = orig })

	vars.APIConfig.STT.Service = "whisper"
	if !useWhisper() {
		t.Error("useWhisper() = false, want true when STT.Service is \"whisper\"")
	}

	vars.APIConfig.STT.Service = "vosk"
	if useWhisper() {
		t.Error("useWhisper() = true, want false when STT.Service is \"vosk\"")
	}

	vars.APIConfig.STT.Service = ""
	if useWhisper() {
		t.Error("useWhisper() = true, want false when STT.Service is unset (should default to vosk)")
	}
}
