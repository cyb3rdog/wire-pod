package botsetup

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

// withTempCertDir points vars.Certs/CertPath/KeyPath/ServerConfigPath at a
// scratch directory for the duration of the test, and restores the
// previous values afterward -- CreateCertCombo/CreateServerConfig write to
// these package-level vars.APIConfig.Server.HostOverride and this the same
// way the real setup flow does.
func withTempCertDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	origCerts, origCertPath, origKeyPath, origServerConfigPath := vars.Certs, vars.CertPath, vars.KeyPath, vars.ServerConfigPath
	origServer := vars.APIConfig.Server

	vars.Certs = dir
	vars.CertPath = filepath.Join(dir, "cert.crt")
	vars.KeyPath = filepath.Join(dir, "cert.key")
	vars.ServerConfigPath = filepath.Join(dir, "server_config.json")

	t.Cleanup(func() {
		vars.Certs, vars.CertPath, vars.KeyPath, vars.ServerConfigPath = origCerts, origCertPath, origKeyPath, origServerConfigPath
		vars.APIConfig.Server = origServer
	})
}

func loadGeneratedCert(t *testing.T) *x509.Certificate {
	t.Helper()
	pemBytes, err := os.ReadFile(vars.CertPath)
	if err != nil {
		t.Fatalf("reading generated cert: %v", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("no PEM block found in generated cert")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing generated cert: %v", err)
	}
	return cert
}

// TestCreateCertComboHostOverrideDNS guards the domain-name path: a
// HostOverride that isn't a literal IP must land in DNSNames, since that's
// what TLS hostname validation actually checks -- an IPAddresses-only SAN
// (the pre-existing, no-override behavior) never matches a domain.
func TestCreateCertComboHostOverrideDNS(t *testing.T) {
	withTempCertDir(t)
	vars.APIConfig.Server.HostOverride = "wirepod.example.com"

	if err := CreateCertCombo(); err != nil {
		t.Fatalf("CreateCertCombo: %v", err)
	}

	cert := loadGeneratedCert(t)
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != "wirepod.example.com" {
		t.Errorf("DNSNames = %v, want [wirepod.example.com]", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 0 {
		t.Errorf("IPAddresses = %v, want none (host override was a domain, not an IP)", cert.IPAddresses)
	}
	if err := cert.VerifyHostname("wirepod.example.com"); err != nil {
		t.Errorf("VerifyHostname(wirepod.example.com) = %v, want nil (this is exactly the check a TLS client performs)", err)
	}
	if err := cert.VerifyHostname("escapepod.local"); err == nil {
		t.Error("VerifyHostname(escapepod.local) = nil, want an error (cert should NOT match an unrelated name)")
	}
}

// TestCreateCertComboHostOverrideIP guards the literal-IP override path,
// which should behave like the original no-override IP mode but pinned to
// the given address instead of the auto-detected outbound one.
func TestCreateCertComboHostOverrideIP(t *testing.T) {
	withTempCertDir(t)
	vars.APIConfig.Server.HostOverride = "203.0.113.7"

	if err := CreateCertCombo(); err != nil {
		t.Fatalf("CreateCertCombo: %v", err)
	}

	cert := loadGeneratedCert(t)
	if len(cert.DNSNames) != 0 {
		t.Errorf("DNSNames = %v, want none (host override was a literal IP)", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 1 || !cert.IPAddresses[0].Equal(net.ParseIP("203.0.113.7")) {
		t.Errorf("IPAddresses = %v, want [203.0.113.7]", cert.IPAddresses)
	}
	if err := cert.VerifyHostname("203.0.113.7"); err != nil {
		t.Errorf("VerifyHostname(203.0.113.7) = %v, want nil", err)
	}
}

// TestCreateCertComboNoOverrideUsesOutboundIP guards the pre-existing,
// no-override default: an empty HostOverride must fall back to the
// machine's own detected IP, exactly as before this feature existed.
func TestCreateCertComboNoOverrideUsesOutboundIP(t *testing.T) {
	withTempCertDir(t)
	vars.APIConfig.Server.HostOverride = ""

	if err := CreateCertCombo(); err != nil {
		t.Fatalf("CreateCertCombo: %v", err)
	}

	cert := loadGeneratedCert(t)
	want := vars.GetOutboundIP()
	if len(cert.IPAddresses) != 1 || !cert.IPAddresses[0].Equal(want) {
		t.Errorf("IPAddresses = %v, want [%v] (the outbound IP)", cert.IPAddresses, want)
	}
	if len(cert.DNSNames) != 0 {
		t.Errorf("DNSNames = %v, want none", cert.DNSNames)
	}
}

// TestCreateServerConfigHostOverride guards the server_config.json side of
// the override: every endpoint field must use the same host (and the
// given port) that CreateCertCombo just put in the cert's SAN, so a
// robot's connection attempt and its TLS validation have a matching
// target -- that pairing is the entire point of this feature.
func TestCreateServerConfigHostOverride(t *testing.T) {
	withTempCertDir(t)
	vars.APIConfig.Server.HostOverride = "wirepod.example.com"
	vars.APIConfig.Server.Port = "8443"
	vars.APIConfig.Server.EPConfig = true // must be overridden regardless

	CreateServerConfig()

	raw, err := os.ReadFile(vars.ServerConfigPath)
	if err != nil {
		t.Fatalf("reading server_config.json: %v", err)
	}
	var got ClientServerConfig
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshaling server_config.json: %v", err)
	}

	want := ClientServerConfig{
		Jdocs:    "wirepod.example.com:8443",
		Token:    "wirepod.example.com:8443",
		Chipper:  "wirepod.example.com:8443",
		Check:    "wirepod.example.com/ok",
		Logfiles: "s3://anki-device-logs-prod/victor",
		Appkey:   "oDoa0quieSeir6goowai7f",
	}
	if got != want {
		t.Errorf("server_config.json = %+v, want %+v", got, want)
	}
}

// TestCreateServerConfigHostOverrideDefaultPort guards the same-port
// contract when Port is left empty: CreateCertCombo has no port to align
// with, but CreateServerConfig must still default to 443 (matching the
// "ep" mode default) rather than writing a bare "host:" endpoint.
func TestCreateServerConfigHostOverrideDefaultPort(t *testing.T) {
	withTempCertDir(t)
	vars.APIConfig.Server.HostOverride = "wirepod.example.com"
	vars.APIConfig.Server.Port = ""

	CreateServerConfig()

	raw, err := os.ReadFile(vars.ServerConfigPath)
	if err != nil {
		t.Fatalf("reading server_config.json: %v", err)
	}
	var got ClientServerConfig
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshaling server_config.json: %v", err)
	}
	if got.Chipper != "wirepod.example.com:443" {
		t.Errorf("Chipper = %q, want wirepod.example.com:443", got.Chipper)
	}
}
