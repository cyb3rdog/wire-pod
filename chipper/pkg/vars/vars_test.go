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
