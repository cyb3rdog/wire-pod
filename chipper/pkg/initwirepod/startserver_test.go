package initwirepod

import (
	"net"
	"testing"

	"github.com/soheilhy/cmux"
)

// withServerState saves/restores the package-level cmux/listener vars that
// closeServers operates on, so tests don't leak state into each other or
// into the real server lifecycle.
func withServerState(t *testing.T) {
	t.Helper()
	origOne, origTwo, origLOne, origLTwo := serverOne, serverTwo, listenerOne, listenerTwo
	t.Cleanup(func() {
		serverOne, serverTwo, listenerOne, listenerTwo = origOne, origTwo, origLOne, origLTwo
	})
}

// TestCloseServersNilSafe guards the actual bug behind the "internal
// error" report: serverTwo/listenerTwo are only ever assigned in
// StartChipper when the running config has the legacy :8084 listener
// active (EPConfig && Port8084Enabled) -- IP mode and Custom Host mode
// never touch them, leaving them at their nil interface zero value.
// Calling .Close() on a nil interface panics unconditionally in Go, and
// RestartServer (which calls this) runs synchronously inside the
// /api-chipper/* HTTP handler, so an unguarded call here surfaced as a
// recovered panic -- "internal error" -- on every reconfigure of an
// already-running non-EPConfig server.
func TestCloseServersNilSafe(t *testing.T) {
	withServerState(t)
	serverOne, serverTwo, listenerOne, listenerTwo = nil, nil, nil, nil

	closeServers() // must not panic
}

// TestCloseServersClosesRealListeners is the flip side: when the values
// legitimately aren't nil, closeServers must still actually close them,
// not skip real work because of an overly broad guard.
func TestCloseServersClosesRealListeners(t *testing.T) {
	withServerState(t)

	lOne, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	lTwo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	listenerOne, listenerTwo = lOne, lTwo
	serverOne, serverTwo = cmux.New(lOne), cmux.New(lTwo)

	closeServers()

	if _, err := lOne.Accept(); err == nil {
		t.Error("listenerOne.Accept() succeeded after closeServers, want an error (listener should be closed)")
	}
	if _, err := lTwo.Accept(); err == nil {
		t.Error("listenerTwo.Accept() succeeded after closeServers, want an error (listener should be closed)")
	}
}

// TestCloseServersPartialNilSafe covers the mixed case that actually
// happens in practice: a server that was started in EPConfig mode (both
// pairs assigned) and then reconfigured to IP/Custom Host mode still has
// serverOne/listenerOne live but never (re)assigns serverTwo/listenerTwo
// on that pass -- see the reconfiguration path this test's sibling in
// certs_test.go exercises via CreateServerConfig.
func TestCloseServersPartialNilSafe(t *testing.T) {
	withServerState(t)

	lOne, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	listenerOne = lOne
	serverOne = cmux.New(lOne)
	serverTwo, listenerTwo = nil, nil

	closeServers() // must not panic despite serverOne/listenerOne being real

	if _, err := lOne.Accept(); err == nil {
		t.Error("listenerOne.Accept() succeeded after closeServers, want an error (listener should be closed)")
	}
}
