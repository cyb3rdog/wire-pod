package vars

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

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
