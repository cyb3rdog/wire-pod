package vars

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// TestSDKEnabledPort80EnabledPort8084EnabledReadAdvancedConfig guards that
// these three accessors are now plain reads of APIConfig.Advanced -- a
// dashboard/apiConfig setting like everything else, not a live env-var
// read. Env vars only ever seed Advanced once, via
// migrateAdvancedSettings (see TestMigrateAdvancedSettings*); these
// functions themselves must not consult the environment at all anymore.
func TestSDKEnabledPort80EnabledPort8084EnabledReadAdvancedConfig(t *testing.T) {
	origAdvanced := APIConfig.Advanced
	t.Cleanup(func() { APIConfig.Advanced = origAdvanced })

	APIConfig.Advanced.SDKEnabled = true
	APIConfig.Advanced.Port80Enabled = true
	APIConfig.Advanced.Port8084Enabled = true
	if !SDKEnabled() || !Port80Enabled() || !Port8084Enabled() {
		t.Error("accessors = false with Advanced fields true, want true")
	}

	APIConfig.Advanced.SDKEnabled = false
	APIConfig.Advanced.Port80Enabled = false
	APIConfig.Advanced.Port8084Enabled = false
	if SDKEnabled() || Port80Enabled() || Port8084Enabled() {
		t.Error("accessors = true with Advanced fields false, want false")
	}

	// Setting the old env vars must have no effect at all now.
	os.Setenv("SDK_ENABLED", "true")
	os.Setenv("PORT80_ENABLED", "true")
	os.Setenv("NO8084", "false") // NO8084=false historically meant "enabled"
	t.Cleanup(func() {
		os.Unsetenv("SDK_ENABLED")
		os.Unsetenv("PORT80_ENABLED")
		os.Unsetenv("NO8084")
	})
	if SDKEnabled() || Port80Enabled() || Port8084Enabled() {
		t.Error("accessors changed based on env vars, want them to only ever read Advanced (env vars only matter during the one-time migration)")
	}
}

// TestMigrateAdvancedSettingsSeedsFromLegacyEnvVars guards the one-time
// upgrade path: an install predating the Advanced struct (AdvancedMigrated
// still false, zero-value fields) must pick up whatever the legacy env
// vars say right now, including NO8084's inverted sense, so existing
// compose.yaml-based deployments don't silently change behavior on
// upgrade.
func TestMigrateAdvancedSettingsSeedsFromLegacyEnvVars(t *testing.T) {
	origAdvanced := APIConfig.Advanced
	origMigrated := APIConfig.AdvancedMigrated
	t.Cleanup(func() {
		APIConfig.Advanced = origAdvanced
		APIConfig.AdvancedMigrated = origMigrated
		for _, v := range []string{"VOSK_THERMAL_ENABLED", "VOSK_WITH_GRAMMER", "DISABLE_MDNS", "NO8084", "JDOCS_PINGER_ENABLED", "SDK_ENABLED", "PORT80_ENABLED"} {
			os.Unsetenv(v)
		}
	})

	APIConfig.Advanced.VoskThermalEnabled = false
	APIConfig.Advanced.VoskWithGrammar = false
	APIConfig.Advanced.DisableMDNS = false
	APIConfig.Advanced.Port8084Enabled = false
	APIConfig.Advanced.JdocsPingerEnabled = false
	APIConfig.Advanced.SDKEnabled = false
	APIConfig.Advanced.Port80Enabled = false
	APIConfig.AdvancedMigrated = false

	os.Setenv("VOSK_THERMAL_ENABLED", "false")
	os.Setenv("VOSK_WITH_GRAMMER", "true")
	os.Setenv("DISABLE_MDNS", "true")
	os.Setenv("NO8084", "true") // inverted: true means disabled
	os.Setenv("JDOCS_PINGER_ENABLED", "false")
	os.Setenv("SDK_ENABLED", "false")
	os.Setenv("PORT80_ENABLED", "false")

	migrateAdvancedSettings()

	if !APIConfig.AdvancedMigrated {
		t.Fatal("AdvancedMigrated = false after migrateAdvancedSettings, want true")
	}
	if APIConfig.Advanced.VoskThermalEnabled {
		t.Error("VoskThermalEnabled = true, want false (VOSK_THERMAL_ENABLED=false)")
	}
	if !APIConfig.Advanced.VoskWithGrammar {
		t.Error("VoskWithGrammar = false, want true (VOSK_WITH_GRAMMER=true)")
	}
	if !APIConfig.Advanced.DisableMDNS {
		t.Error("DisableMDNS = false, want true (DISABLE_MDNS=true)")
	}
	if APIConfig.Advanced.Port8084Enabled {
		t.Error("Port8084Enabled = true, want false (NO8084=true, inverted sense)")
	}
	if APIConfig.Advanced.JdocsPingerEnabled {
		t.Error("JdocsPingerEnabled = true, want false (JDOCS_PINGER_ENABLED=false)")
	}
	if APIConfig.Advanced.SDKEnabled {
		t.Error("SDKEnabled = true, want false (SDK_ENABLED=false)")
	}
	if APIConfig.Advanced.Port80Enabled {
		t.Error("Port80Enabled = true, want false (PORT80_ENABLED=false)")
	}
}

// TestMigrateAdvancedSettingsRunsOnce guards the migration flag itself:
// once AdvancedMigrated is true, changing env vars and calling
// migrateAdvancedSettings again must be a no-op -- this is what makes
// Advanced dashboard-authoritative after the first boot, exactly like
// every other setting in this struct.
func TestMigrateAdvancedSettingsRunsOnce(t *testing.T) {
	origAdvanced := APIConfig.Advanced
	origMigrated := APIConfig.AdvancedMigrated
	t.Cleanup(func() {
		APIConfig.Advanced = origAdvanced
		APIConfig.AdvancedMigrated = origMigrated
		os.Unsetenv("SDK_ENABLED")
	})

	APIConfig.AdvancedMigrated = true
	APIConfig.Advanced.SDKEnabled = true // simulating a value set via the dashboard

	os.Setenv("SDK_ENABLED", "false") // stale env var, should be ignored now
	migrateAdvancedSettings()

	if !APIConfig.Advanced.SDKEnabled {
		t.Error("migrateAdvancedSettings overwrote an already-migrated config from a stale env var")
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
		APIConfig = apiConfig{}
		ApiConfigPath = origPath
	})

	os.Setenv("HOST_OVERRIDE", "wirepod.example.com")
	os.Setenv("SERVER_PORT", "8443")
	APIConfig = apiConfig{}
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
		APIConfig = apiConfig{}
		ApiConfigPath = origPath
	})

	os.Setenv("HOST_OVERRIDE", "wirepod.example.com")
	os.Unsetenv("SERVER_PORT")
	APIConfig = apiConfig{}
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
		APIConfig = apiConfig{}
		ApiConfigPath = origPath
	})

	os.Unsetenv("HOST_OVERRIDE")
	os.Unsetenv("SERVER_PORT")
	APIConfig = apiConfig{}
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
