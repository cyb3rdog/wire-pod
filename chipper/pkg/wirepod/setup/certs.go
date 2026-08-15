package botsetup

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

type ClientServerConfig struct {
	Jdocs    string `json:"jdocs"`
	Token    string `json:"tms"`
	Chipper  string `json:"chipper"`
	Check    string `json:"check"`
	Logfiles string `json:"logfiles"`
	Appkey   string `json:"appkey"`
}

// creates and exports a priv/pub key combo generated with IP address
func CreateCertCombo() error {
	// ca certificate
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(2019),
		Subject:               pkix.Name{},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(30, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 1028)
	if err != nil {
		return err
	}

	// create actual certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject:      pkix.Name{},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	// HostOverride wins when set: if it parses as a literal IP, it goes
	// in IPAddresses (same as the default path below); otherwise it's a
	// hostname/domain, which needs a DNSNames SAN instead -- TLS clients
	// validate hostname-based connections against DNSNames, not
	// IPAddresses, so a domain here would never match an IP-only SAN.
	// With no override, fall back to the historical behavior: the
	// machine's own outbound-facing local IP.
	if host := strings.TrimSpace(vars.GetAPIConfig().Server.HostOverride); host != "" {
		if ip := net.ParseIP(host); ip != nil {
			cert.IPAddresses = []net.IP{ip}
		} else {
			cert.DNSNames = []string{host}
		}
	} else {
		cert.IPAddresses = []net.IP{vars.GetOutboundIP()}
	}
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 1028)
	if err != nil {
		return err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}
	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	// export certificates
	os.MkdirAll(vars.Certs, 0777)
	logger.Println("Outputting certificate to " + vars.CertPath)
	err = os.WriteFile(vars.CertPath, certPEM.Bytes(), 0777)
	if err != nil {
		return err
	}
	logger.Println("Outputting private key to " + vars.KeyPath)
	err = os.WriteFile(vars.KeyPath, certPrivKeyPEM.Bytes(), 0777)
	if err != nil {
		return err
	}
	vars.ChipperCert = certPEM.Bytes()
	vars.ChipperKey = certPrivKeyPEM.Bytes()
	vars.ChipperKeysLoaded = true

	return nil
}

// outputs a server config to ../certs/server_config.json
func CreateServerConfig() {
	os.MkdirAll(vars.Certs, 0777)
	var config ClientServerConfig
	//{"jdocs": "escapepod.local:443", "tms": "escapepod.local:443", "chipper": "escapepod.local:443", "check": "escapepod.local/ok:80", "logfiles": "s3://anki-device-logs-prod/victor", "appkey": "oDoa0quieSeir6goowai7f"}
	// One snapshot for the whole switch below, not three separate reads
	// of HostOverride/Port/EPConfig -- a settings change landing between
	// them used to risk writing a server_config.json that mixed old and
	// new values (e.g. the new HostOverride's host with the old Port).
	server := vars.GetAPIConfig().Server
	host := strings.TrimSpace(server.HostOverride)
	switch {
	case host != "":
		// Same value CreateCertCombo just put in the cert's SAN, so the
		// robot's TLS validation of this address actually has something
		// to match against.
		port := server.Port
		if port == "" {
			port = "443"
		}
		url := host + ":" + port
		config.Jdocs = url
		config.Token = url
		config.Chipper = url
		config.Check = host + "/ok"
		config.Logfiles = "s3://anki-device-logs-prod/victor"
		config.Appkey = "oDoa0quieSeir6goowai7f"
	case server.EPConfig:
		config.Jdocs = "escapepod.local:443"
		config.Token = "escapepod.local:443"
		config.Chipper = "escapepod.local:443"
		config.Check = "escapepod.local/ok"
		config.Logfiles = "s3://anki-device-logs-prod/victor"
		config.Appkey = "oDoa0quieSeir6goowai7f"
	default:
		ip := vars.GetOutboundIP()
		ipString := ip.String()
		url := ipString + ":" + server.Port
		config.Jdocs = url
		config.Token = url
		config.Chipper = url
		config.Check = ipString + "/ok"
		config.Logfiles = "s3://anki-device-logs-prod/victor"
		config.Appkey = "oDoa0quieSeir6goowai7f"
	}
	writeBytes, _ := json.Marshal(config)
	os.WriteFile(vars.ServerConfigPath, writeBytes, 0777)
}
