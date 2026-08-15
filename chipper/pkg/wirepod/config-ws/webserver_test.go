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

	// A valid, passes-validation body -- this test is specifically about
	// the disk write failing, not about request validation (see
	// TestSetKGAPIRejectsInvalidRequests for that).
	body := `{"enable": true, "provider": "custom", "key": "sk-test-key", "endpoint": "https://my-llm.example.com/v1"}`
	req := httptest.NewRequest("POST", "/api/set_kg_api", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handleSetKGAPI(rec, req)

	if rec.Code == 200 {
		t.Fatalf("handleSetKGAPI returned 200 despite the disk write failing -- the caller has no way to know the setting won't survive a restart. Body: %s", rec.Body.String())
	}
}

// TestSetKGAPIRejectsInvalidRequests guards the validation gap this
// handler used to have: it decoded straight into live config with no
// checks at all, unlike every other settings handler in this file.
func TestSetKGAPIRejectsInvalidRequests(t *testing.T) {
	dir := t.TempDir()
	origPath := vars.ApiConfigPath
	origKnowledge := vars.APIConfig.Knowledge
	t.Cleanup(func() {
		vars.ApiConfigPath = origPath
		vars.APIConfig.Knowledge = origKnowledge
	})
	vars.ApiConfigPath = filepath.Join(dir, "apiConfig.json")

	cases := []struct {
		name string
		body string
	}{
		{"unknown provider", `{"enable": true, "provider": "not-a-real-provider", "key": "sk-test-key"}`},
		{"empty provider while enabled", `{"enable": true, "provider": "", "key": "sk-test-key"}`},
		{"missing key while enabled", `{"enable": true, "provider": "openai", "key": ""}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vars.APIConfig.Knowledge = origKnowledge
			req := httptest.NewRequest("POST", "/api/set_kg_api", strings.NewReader(c.body))
			rec := httptest.NewRecorder()
			handleSetKGAPI(rec, req)
			if rec.Code != 400 {
				t.Fatalf("expected 400 for %s, got %d: %s", c.name, rec.Code, rec.Body.String())
			}
			if vars.APIConfig.Knowledge.Enable {
				t.Fatalf("%s: live config was mutated despite the request being rejected", c.name)
			}
		})
	}

	// Disabling should never require a provider/key.
	req := httptest.NewRequest("POST", "/api/set_kg_api", strings.NewReader(`{"enable": false, "provider": "", "key": ""}`))
	rec := httptest.NewRecorder()
	handleSetKGAPI(rec, req)
	if rec.Code != 200 {
		t.Fatalf("disabling knowledge graph should not require validation, got %d: %s", rec.Code, rec.Body.String())
	}
}
