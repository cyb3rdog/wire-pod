package vars

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// TestSDKEnabledDefaultsOnAndRespectsFalse guards the SDK app server's
// off switch: unset (or any value other than "false") must default to
// enabled, matching existing behavior for anyone who never set this var.
func TestSDKEnabledDefaultsOnAndRespectsFalse(t *testing.T) {
	t.Cleanup(func() { os.Unsetenv("SDK_ENABLED") })

	os.Unsetenv("SDK_ENABLED")
	if !SDKEnabled() {
		t.Error("SDKEnabled() = false with the env var unset, want true (default on)")
	}

	os.Setenv("SDK_ENABLED", "true")
	if !SDKEnabled() {
		t.Error("SDKEnabled() = false with SDK_ENABLED=true, want true")
	}

	os.Setenv("SDK_ENABLED", "false")
	if SDKEnabled() {
		t.Error("SDKEnabled() = true with SDK_ENABLED=false, want false")
	}
}

// TestPort80EnabledDefaultsOnAndRespectsFalse guards the same pattern for
// the port-80 socket toggle, which is deliberately independent of
// SDKEnabled (see sdkapp.BeginServer).
func TestPort80EnabledDefaultsOnAndRespectsFalse(t *testing.T) {
	t.Cleanup(func() { os.Unsetenv("PORT80_ENABLED") })

	os.Unsetenv("PORT80_ENABLED")
	if !Port80Enabled() {
		t.Error("Port80Enabled() = false with the env var unset, want true (default on)")
	}

	os.Setenv("PORT80_ENABLED", "true")
	if !Port80Enabled() {
		t.Error("Port80Enabled() = false with PORT80_ENABLED=true, want true")
	}

	os.Setenv("PORT80_ENABLED", "false")
	if Port80Enabled() {
		t.Error("Port80Enabled() = true with PORT80_ENABLED=false, want false")
	}
}

// TestPort8084EnabledInvertedSense guards NO8084's semantics, which are
// deliberately inverted relative to every other toggle in this file
// (true means disabled, not enabled) -- see the doc comment on
// Port8084Enabled for why this var was kept as-is instead of migrated.
func TestPort8084EnabledInvertedSense(t *testing.T) {
	t.Cleanup(func() { os.Unsetenv("NO8084") })

	os.Unsetenv("NO8084")
	if !Port8084Enabled() {
		t.Error("Port8084Enabled() = false with NO8084 unset, want true (default on)")
	}

	os.Setenv("NO8084", "true")
	if Port8084Enabled() {
		t.Error("Port8084Enabled() = true with NO8084=true, want false")
	}

	os.Setenv("NO8084", "false")
	if !Port8084Enabled() {
		t.Error("Port8084Enabled() = false with NO8084=false, want true")
	}
}

// TestCreateConfigFromEnvHostOverride guards the declarative Custom Host
// deploy path: HOST_OVERRIDE should seed Server.HostOverride/EPConfig/
// Port and, critically, PastInitialSetup=true -- without that last part,
// StartFromProgramInit would still refuse to start the chipper server and
// insist on a trip through initial.html despite the connection being
// fully specified already.
func TestCreateConfigFromEnvHostOverride(t *testing.T) {
	origPath := ApiConfigPath
	ApiConfigPath = t.TempDir() + "/apiConfig.json"
	t.Cleanup(func() {
		os.Unsetenv("HOST_OVERRIDE")
		os.Unsetenv("SERVER_PORT")
		APIConfig = Config{}
		ApiConfigPath = origPath
	})

	os.Setenv("HOST_OVERRIDE", "wirepod.example.com")
	os.Setenv("SERVER_PORT", "8443")
	APIConfig = Config{}
	CreateConfigFromEnv()

	if APIConfig.Server.HostOverride != "wirepod.example.com" {
		t.Errorf("HostOverride = %q, want wirepod.example.com", APIConfig.Server.HostOverride)
	}
	if APIConfig.Server.EPConfig {
		t.Error("EPConfig = true, want false when HOST_OVERRIDE is set")
	}
	if APIConfig.Server.Port != "8443" {
		t.Errorf("Port = %q, want 8443", APIConfig.Server.Port)
	}
	if !APIConfig.PastInitialSetup {
		t.Error("PastInitialSetup = false, want true -- HOST_OVERRIDE fully specifies the connection, no wizard trip needed")
	}
}

// TestCreateConfigFromEnvHostOverrideDefaultPort guards the port default:
// unset SERVER_PORT must fall back to 443, matching compose.yaml's
// default published port so the common case doesn't silently need an
// unpublished port.
func TestCreateConfigFromEnvHostOverrideDefaultPort(t *testing.T) {
	origPath := ApiConfigPath
	ApiConfigPath = t.TempDir() + "/apiConfig.json"
	t.Cleanup(func() {
		os.Unsetenv("HOST_OVERRIDE")
		APIConfig = Config{}
		ApiConfigPath = origPath
	})

	os.Setenv("HOST_OVERRIDE", "wirepod.example.com")
	os.Unsetenv("SERVER_PORT")
	APIConfig = Config{}
	CreateConfigFromEnv()

	if APIConfig.Server.Port != "443" {
		t.Errorf("Port = %q, want 443 (default)", APIConfig.Server.Port)
	}
}

// TestCreateConfigFromEnvNoHostOverride guards the unchanged default:
// without HOST_OVERRIDE, Server fields must stay at their zero value --
// existing Escape Pod/IP-mode deployments (and the wizard flow) must see
// no behavior change at all.
func TestCreateConfigFromEnvNoHostOverride(t *testing.T) {
	origPath := ApiConfigPath
	ApiConfigPath = t.TempDir() + "/apiConfig.json"
	t.Cleanup(func() {
		APIConfig = Config{}
		ApiConfigPath = origPath
	})

	os.Unsetenv("HOST_OVERRIDE")
	os.Unsetenv("SERVER_PORT")
	APIConfig = Config{}
	CreateConfigFromEnv()

	if APIConfig.Server.HostOverride != "" {
		t.Errorf("HostOverride = %q, want empty", APIConfig.Server.HostOverride)
	}
	if APIConfig.Server.EPConfig {
		t.Error("EPConfig = true, want false (zero value) with no env vars set")
	}
	if APIConfig.PastInitialSetup {
		t.Error("PastInitialSetup = true, want false -- still needs the wizard without HOST_OVERRIDE")
	}
}

// TestDeleteDataNoDeadlock guards against a regression of the
// DeleteData -> WriteJdocs reentrant-lock deadlock: DeleteData held
// botJdocsMu.Lock() and then called WriteJdocs(), which locked the
// same non-reentrant mutex again, hanging the goroutine forever.
func TestDeleteDataNoDeadlock(t *testing.T) {
	JdocsPath = t.TempDir() + "/jdocs.json"
	BotJdocs = []botjdoc{{Thing: "vic:00000001"}}

	done := make(chan struct{})
	go func() {
		DeleteData("vic:00000001")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DeleteData deadlocked")
	}

	if len(BotJdocs) != 0 {
		t.Fatalf("expected BotJdocs to be empty after delete, got %d entries", len(BotJdocs))
	}
}

// TestAddJdocNoDeadlock is a companion check for AddJdoc, which also
// persists through the shared writeJdocsLocked helper.
func TestAddJdocNoDeadlock(t *testing.T) {
	JdocsPath = t.TempDir() + "/jdocs.json"
	BotJdocs = nil

	done := make(chan struct{})
	go func() {
		AddJdoc("vic:00000002", "vic.RobotSettings", AJdoc{DocVersion: 1})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AddJdoc deadlocked")
	}
}

// TestBotInfoConcurrentAccess exercises GetBotInfo/UpdateBotInfo from
// many goroutines at once. Run with -race to catch a regression of
// the unsynchronized BotInfo global (concurrent append/read on the
// same slice header).
func TestBotInfoConcurrentAccess(t *testing.T) {
	BotInfoPath = t.TempDir() + "/botSdkInfo.json"
	BotInfo = RobotInfoStore{}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		n := i
		go func() {
			defer wg.Done()
			UpdateBotInfo(func(bi *RobotInfoStore) {
				bi.Robots = append(bi.Robots, RobotEntry{Esn: fmt.Sprintf("esn%d", n)})
			})
		}()
		go func() {
			defer wg.Done()
			_ = GetBotInfo()
		}()
	}
	wg.Wait()

	if len(BotInfo.Robots) != 50 {
		t.Fatalf("expected 50 robots after concurrent updates, got %d", len(BotInfo.Robots))
	}
}

// TestAPIConfigConcurrentAccess exercises GetAPIConfig/UpdateAPIConfig
// from many goroutines at once -- the same pattern as
// TestBotInfoConcurrentAccess above, for the config struct that spent
// this whole codebase's history as a raw package-level var read and
// written directly from ~170 call sites across the backend (dashboard
// HTTP handlers racing per-robot LLM/STT request goroutines) with zero
// synchronization at all. Run with -race: it fails hard against the
// direct-field-access version this replaced (confirmed by temporarily
// reverting GetAPIConfig/UpdateAPIConfig to plain field access and
// re-running with -race during development of this fix) and passes
// clean against the mutex-guarded version.
func TestAPIConfigConcurrentAccess(t *testing.T) {
	ApiConfigPath = t.TempDir() + "/apiConfig.json"
	APIConfig = Config{}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		n := i
		go func() {
			defer wg.Done()
			UpdateAPIConfig(func(cfg *Config) {
				cfg.Knowledge.Key = fmt.Sprintf("key%d", n)
			})
		}()
		go func() {
			defer wg.Done()
			_ = GetAPIConfig()
		}()
		go func() {
			defer wg.Done()
			SetAPIConfigInMemory(func(cfg *Config) {
				cfg.PastInitialSetup = n%2 == 0
			})
		}()
	}
	wg.Wait()
}

// TestCustomIntentsConcurrentAccess is the same pattern for CustomIntents:
// dashboard add/edit/remove-intent HTTP handlers used to mutate it
// directly while every robot's intent matching (ttr.customIntentHandler,
// stt/vosk's grammar builder) read it concurrently, with no
// synchronization. Run with -race.
func TestCustomIntentsConcurrentAccess(t *testing.T) {
	CustomIntentsPath = t.TempDir() + "/customIntents.json"
	CustomIntents = nil

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		n := i
		go func() {
			defer wg.Done()
			UpdateCustomIntents(func(intents *[]CustomIntent) {
				*intents = append(*intents, CustomIntent{
					Name:       fmt.Sprintf("intent%d", n),
					Utterances: []string{"hello", "hi"},
				})
			})
		}()
		go func() {
			defer wg.Done()
			for _, ci := range GetCustomIntents() {
				_ = ci.Utterances
			}
		}()
	}
	wg.Wait()

	if len(CustomIntents) != 50 {
		t.Fatalf("expected 50 custom intents after concurrent updates, got %d", len(CustomIntents))
	}
}
