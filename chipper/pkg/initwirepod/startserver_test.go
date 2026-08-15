package initwirepod

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
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

func withTempVarsPaths(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	origCerts, origCertPath, origKeyPath, origServerConfigPath := vars.Certs, vars.CertPath, vars.KeyPath, vars.ServerConfigPath
	origApiConfigPath := vars.ApiConfigPath
	origServer := vars.APIConfig.Server
	t.Cleanup(func() {
		vars.Certs, vars.CertPath, vars.KeyPath, vars.ServerConfigPath = origCerts, origCertPath, origKeyPath, origServerConfigPath
		vars.ApiConfigPath = origApiConfigPath
		vars.APIConfig.Server = origServer
	})
	vars.Certs = filepath.Join(dir, "certs")
	vars.CertPath = filepath.Join(dir, "certs", "cert.crt")
	vars.KeyPath = filepath.Join(dir, "certs", "cert.key")
	vars.ServerConfigPath = filepath.Join(dir, "certs", "server_config.json")
	// ensureCertForHostOverride's success path calls vars.WriteConfigToDisk,
	// which would otherwise write to the real default ("./apiConfig.json",
	// relative to the test binary's CWD) and leave a stray file in the
	// source tree.
	vars.ApiConfigPath = filepath.Join(dir, "apiConfig.json")
}

// TestEnsureCertForHostOverrideGeneratesCert is the end-to-end proof for
// the declarative deploy path: HOST_OVERRIDE seeds Server.HostOverride
// (vars.CreateConfigFromEnv), and this is what's supposed to turn that
// into an actual, correctly-SAN'd cert before the first StartChipper call
// -- no trip through initial.html required.
func TestEnsureCertForHostOverrideGeneratesCert(t *testing.T) {
	withTempVarsPaths(t)
	vars.APIConfig.Server.HostOverride = "wirepod.example.com"
	vars.APIConfig.Server.Port = "443"
	vars.APIConfig.Server.EPConfig = false

	ensureCertForHostOverride()

	pemBytes, err := os.ReadFile(vars.CertPath)
	if err != nil {
		t.Fatalf("cert was not generated: %v", err)
	}
	block, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing generated cert: %v", err)
	}
	if err := cert.VerifyHostname("wirepod.example.com"); err != nil {
		t.Errorf("VerifyHostname(wirepod.example.com) = %v, want nil", err)
	}

	if _, err := os.Stat(vars.ServerConfigPath); err != nil {
		t.Errorf("server_config.json was not generated: %v", err)
	}
}

// TestEnsureCertForHostOverrideSkipsWhenCertExists guards against
// clobbering a cert that's already correct for the current config on
// every subsequent restart -- HOST_OVERRIDE (like every other env-seeded
// setting) should only ever take effect on a genuinely fresh setup.
func TestEnsureCertForHostOverrideSkipsWhenCertExists(t *testing.T) {
	withTempVarsPaths(t)
	vars.APIConfig.Server.HostOverride = "wirepod.example.com"
	os.MkdirAll(vars.Certs, 0777)
	const sentinel = "not a real cert, just proving this wasn't touched"
	if err := os.WriteFile(vars.CertPath, []byte(sentinel), 0644); err != nil {
		t.Fatalf("seeding existing cert file: %v", err)
	}

	ensureCertForHostOverride()

	got, err := os.ReadFile(vars.CertPath)
	if err != nil {
		t.Fatalf("reading cert: %v", err)
	}
	if string(got) != sentinel {
		t.Error("ensureCertForHostOverride regenerated an already-existing cert, want it left untouched")
	}
}

// TestEnsureCertForHostOverrideNoopWithoutOverride guards the common
// case (Escape Pod/plain IP mode, no HOST_OVERRIDE set at all) -- must do
// nothing, not even touch the filesystem.
func TestEnsureCertForHostOverrideNoopWithoutOverride(t *testing.T) {
	withTempVarsPaths(t)
	vars.APIConfig.Server.HostOverride = ""

	ensureCertForHostOverride()

	if _, err := os.Stat(vars.CertPath); err == nil {
		t.Error("ensureCertForHostOverride generated a cert with no HostOverride set")
	}
}
