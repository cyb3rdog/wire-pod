package wirepod_whisper

import (
	"os"
	"testing"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

func resetConfig(t *testing.T) {
	t.Helper()
	vars.APIConfig.STT.Whisper.BaseURL = ""
	vars.APIConfig.STT.Whisper.APIKey = ""
	vars.APIConfig.STT.Whisper.Model = ""
	t.Cleanup(func() {
		vars.APIConfig.STT.Whisper.BaseURL = ""
		vars.APIConfig.STT.Whisper.APIKey = ""
		vars.APIConfig.STT.Whisper.Model = ""
		os.Unsetenv("OPENAI_KEY")
	})
}

func TestResolvedBaseURLDefaultsToOpenAI(t *testing.T) {
	resetConfig(t)
	if got := resolvedBaseURL(); got != defaultBaseURL {
		t.Errorf("resolvedBaseURL() = %q, want %q", got, defaultBaseURL)
	}
}

func TestResolvedBaseURLUsesConfigAndTrimsSlash(t *testing.T) {
	resetConfig(t)
	vars.APIConfig.STT.Whisper.BaseURL = "http://faster-whisper:8000/"
	if got, want := resolvedBaseURL(), "http://faster-whisper:8000"; got != want {
		t.Errorf("resolvedBaseURL() = %q, want %q", got, want)
	}
}

func TestResolvedModelDefaultsToWhisper1(t *testing.T) {
	resetConfig(t)
	if got := resolvedModel(); got != defaultModel {
		t.Errorf("resolvedModel() = %q, want %q", got, defaultModel)
	}
}

func TestResolvedModelUsesConfig(t *testing.T) {
	resetConfig(t)
	vars.APIConfig.STT.Whisper.Model = "large-v3"
	if got, want := resolvedModel(), "large-v3"; got != want {
		t.Errorf("resolvedModel() = %q, want %q", got, want)
	}
}

func TestResolvedAPIKeyPrefersConfigOverEnv(t *testing.T) {
	resetConfig(t)
	os.Setenv("OPENAI_KEY", "env-key")
	vars.APIConfig.STT.Whisper.APIKey = "config-key"
	if got, want := resolvedAPIKey(), "config-key"; got != want {
		t.Errorf("resolvedAPIKey() = %q, want %q", got, want)
	}
}

func TestResolvedAPIKeyFallsBackToEnv(t *testing.T) {
	resetConfig(t)
	os.Setenv("OPENAI_KEY", "env-key")
	if got, want := resolvedAPIKey(), "env-key"; got != want {
		t.Errorf("resolvedAPIKey() = %q, want %q", got, want)
	}
}
