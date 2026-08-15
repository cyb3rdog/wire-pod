package webserver

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

// TestSetKGAPIPersistsAcrossRestart guards the reported symptom: Knowledge
// Graph settings saved from the dashboard (handleSetKGAPI, exactly the
// request main.js's sendKGAPIKey() sends for the "custom" LLM provider)
// must still be there after a simulated container restart -- wiping
// in-memory state and re-reading apiConfig.json from disk, exactly what
// vars.Init() -> vars.ReadConfig() does on every real process start.
func TestSetKGAPIPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	origPath := vars.ApiConfigPath
	origKnowledge := vars.APIConfig.Knowledge
	t.Cleanup(func() {
		vars.ApiConfigPath = origPath
		vars.APIConfig.Knowledge = origKnowledge
	})
	vars.ApiConfigPath = filepath.Join(dir, "apiConfig.json")

	body := `{
		"enable": true,
		"provider": "custom",
		"key": "sk-test-key",
		"model": "my-model",
		"id": "",
		"intentgraph": false,
		"robotName": "",
		"openai_prompt": "You are Vector.",
		"openai_voice": "",
		"openai_voice_with_english": false,
		"save_chat": false,
		"commands_enable": false,
		"endpoint": "https://my-llm.example.com/v1"
	}`
	req := httptest.NewRequest("POST", "/api/set_kg_api", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handleSetKGAPI(rec, req)

	if rec.Code != 200 {
		t.Fatalf("handleSetKGAPI returned status %d, body: %s", rec.Code, rec.Body.String())
	}
	if vars.APIConfig.Knowledge.Endpoint != "https://my-llm.example.com/v1" {
		t.Fatalf("in-memory Endpoint = %q immediately after save, want the submitted value", vars.APIConfig.Knowledge.Endpoint)
	}

	// Simulate a full process/container restart: deliberately clobber the
	// in-memory fields to WRONG values first, then re-read from disk
	// exactly like vars.Init() -> vars.ReadConfig() does on every real
	// process start.
	vars.APIConfig.Knowledge.Provider = "should-be-overwritten"
	vars.APIConfig.Knowledge.Endpoint = "should-be-overwritten"
	vars.APIConfig.Knowledge.Key = "should-be-overwritten"
	vars.APIConfig.Knowledge.Model = "should-be-overwritten"
	vars.APIConfig.Knowledge.Enable = false

	vars.ReadConfig()

	if !vars.APIConfig.Knowledge.Enable {
		t.Error("after restart: Enable = false, want true")
	}
	if vars.APIConfig.Knowledge.Provider != "custom" {
		t.Errorf("after restart: Provider = %q, want custom", vars.APIConfig.Knowledge.Provider)
	}
	if vars.APIConfig.Knowledge.Endpoint != "https://my-llm.example.com/v1" {
		t.Errorf("after restart: Endpoint = %q, want https://my-llm.example.com/v1", vars.APIConfig.Knowledge.Endpoint)
	}
	if vars.APIConfig.Knowledge.Key != "sk-test-key" {
		t.Errorf("after restart: Key = %q, want sk-test-key", vars.APIConfig.Knowledge.Key)
	}
	if vars.APIConfig.Knowledge.Model != "my-model" {
		t.Errorf("after restart: Model = %q, want my-model", vars.APIConfig.Knowledge.Model)
	}
}

// TestSetKGAPIReportsDiskWriteFailure guards the actual root cause behind
// "settings don't survive a restart": a failed disk write (e.g. a
// permissions problem on the bind-mounted data directory) used to be
// swallowed -- logged, but the handler still told the browser "Changes
// successfully applied." with a 200. It must now report the failure
// instead of claiming success.
func TestSetKGAPIReportsDiskWriteFailure(t *testing.T) {
	origPath := vars.ApiConfigPath
	origKnowledge := vars.APIConfig.Knowledge
	t.Cleanup(func() {
		vars.ApiConfigPath = origPath
		vars.APIConfig.Knowledge = origKnowledge
	})
	// A parent directory that doesn't exist and won't be created --
	// WriteFileAtomic's os.CreateTemp in that directory fails
	// unconditionally, independent of process privilege.
	vars.ApiConfigPath = "/this-directory-does-not-exist-xyz/apiConfig.json"

	body := `{"enable": true, "provider": "custom", "endpoint": "https://my-llm.example.com/v1"}`
	req := httptest.NewRequest("POST", "/api/set_kg_api", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handleSetKGAPI(rec, req)

	if rec.Code == 200 {
		t.Fatalf("handleSetKGAPI returned 200 despite the disk write failing -- the caller has no way to know the setting won't survive a restart. Body: %s", rec.Body.String())
	}
}

// TestSetAdvancedSettingsPersistsAcrossRestart guards the dashboard path
// for the settings that used to be env-var-only with no persistence at
// all (SDK_ENABLED, PORT80_ENABLED, NO8084, DISABLE_MDNS,
// JDOCS_PINGER_ENABLED, VOSK_THERMAL_ENABLED, VOSK_WITH_GRAMMER) -- must
// round-trip through apiConfig.json exactly like every other setting.
func TestSetAdvancedSettingsPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	origPath := vars.ApiConfigPath
	origAdvanced := vars.APIConfig.Advanced
	origMigrated := vars.APIConfig.AdvancedMigrated
	t.Cleanup(func() {
		vars.ApiConfigPath = origPath
		vars.APIConfig.Advanced = origAdvanced
		vars.APIConfig.AdvancedMigrated = origMigrated
	})
	vars.ApiConfigPath = filepath.Join(dir, "apiConfig.json")

	body := `{
		"vosk_thermal_enabled": false,
		"vosk_with_grammar": true,
		"disable_mdns": true,
		"port8084_enabled": false,
		"jdocs_pinger_enabled": false,
		"sdk_enabled": false,
		"port80_enabled": false
	}`
	req := httptest.NewRequest("POST", "/api/set_advanced_settings", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handleSetAdvancedSettings(rec, req)

	if rec.Code != 200 {
		t.Fatalf("handleSetAdvancedSettings returned status %d, body: %s", rec.Code, rec.Body.String())
	}

	// Simulate a full process/container restart: deliberately clobber the
	// in-memory fields to WRONG values first, then re-read from disk
	// exactly like vars.Init() -> vars.ReadConfig() does.
	vars.APIConfig.Advanced.VoskThermalEnabled = true
	vars.APIConfig.Advanced.VoskWithGrammar = false
	vars.APIConfig.Advanced.DisableMDNS = false
	vars.APIConfig.Advanced.Port8084Enabled = true
	vars.APIConfig.Advanced.JdocsPingerEnabled = true
	vars.APIConfig.Advanced.SDKEnabled = true
	vars.APIConfig.Advanced.Port80Enabled = true

	vars.ReadConfig()

	if vars.APIConfig.Advanced.VoskThermalEnabled {
		t.Error("after restart: VoskThermalEnabled = true, want false")
	}
	if !vars.APIConfig.Advanced.VoskWithGrammar {
		t.Error("after restart: VoskWithGrammar = false, want true")
	}
	if !vars.APIConfig.Advanced.DisableMDNS {
		t.Error("after restart: DisableMDNS = false, want true")
	}
	if vars.APIConfig.Advanced.Port8084Enabled {
		t.Error("after restart: Port8084Enabled = true, want false")
	}
	if vars.APIConfig.Advanced.JdocsPingerEnabled {
		t.Error("after restart: JdocsPingerEnabled = true, want false")
	}
	if vars.APIConfig.Advanced.SDKEnabled {
		t.Error("after restart: SDKEnabled = true, want false")
	}
	if vars.APIConfig.Advanced.Port80Enabled {
		t.Error("after restart: Port80Enabled = true, want false")
	}
	// A pre-migrated config must not get re-seeded from env vars on a
	// later restart -- confirms migrateAdvancedSettings' guard is
	// actually reached via the real ReadConfig path, not just tested in
	// isolation.
	if !vars.APIConfig.AdvancedMigrated {
		t.Error("after restart: AdvancedMigrated = false, want true")
	}
}
