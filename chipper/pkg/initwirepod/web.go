package initwirepod

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	botsetup "github.com/kercre123/wire-pod/chipper/pkg/wirepod/setup"
)

// Rate limiter: 10 requests per minute per IP
type rateLimiter struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	limit    int
	window   time.Duration
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    10,
		window:   time.Minute,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Clean old requests
	var valid []time.Time
	for _, t := range rl.requests[ip] {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[ip] = valid
		return false
	}

	rl.requests[ip] = append(valid, now)
	return true
}

var limiter = newRateLimiter()

// cant be part of config-ws, otherwise import cycle

func ChipperHTTPApi(w http.ResponseWriter, r *http.Request) {
	// Rate limiting
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ip = host
	}
	if !limiter.allow(ip) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	switch {
	case r.URL.Path == "/api-chipper/restart":
		RestartServer()
		fmt.Fprint(w, "done")
		return
	case r.URL.Path == "/api-chipper/use_ip":
		port := r.FormValue("port")
		if port == "" {
			fmt.Fprint(w, "error: must have port")
			return
		}
		if _, err := strconv.Atoi(port); err != nil {
			fmt.Fprint(w, "error: port is invalid")
			return
		}
		// IP mode needs a cert whose SAN actually matches the address the
		// robot will dial, so it's regenerated here.
		if err := applyServerConfig(false, port, "", true); err != nil {
			logger.Println(err)
			fmt.Fprint(w, "error: "+err.Error())
			return
		}
		fmt.Fprint(w, "done")
		return
	case r.URL.Path == "/api-chipper/use_ep":
		// Escape Pod mode is resolved by the robot via mDNS against the
		// hostname itself, not a specific IP/host baked into the cert's
		// SAN, so -- unlike the other two modes -- this deliberately does
		// not regenerate the cert.
		if err := applyServerConfig(true, "443", "", false); err != nil {
			logger.Println(err)
			fmt.Fprint(w, "error: "+err.Error())
			return
		}
		fmt.Fprint(w, "done")
		return
	case r.URL.Path == "/api-chipper/use_custom":
		// A robot reached through a domain (or a public IP) that isn't
		// "escapepod.local" and isn't the wire-pod host's own detected
		// local IP -- e.g. port-forwarded behind a reverse proxy. The
		// same value goes into both server_config.json (what the robot
		// dials) and the cert's SAN (what it validates that dial
		// against), which is the part neither "ep" nor plain "ip" mode
		// can express.
		host := strings.TrimSpace(r.FormValue("host"))
		if host == "" {
			fmt.Fprint(w, "error: must have host")
			return
		}
		if strings.ContainsAny(host, "/ \t") {
			fmt.Fprint(w, "error: host must not contain a scheme, path, or spaces -- just a domain or IP")
			return
		}
		port := r.FormValue("port")
		if port == "" {
			port = "443"
		}
		if _, err := strconv.Atoi(port); err != nil {
			fmt.Fprint(w, "error: port is invalid")
			return
		}
		if err := applyServerConfig(false, port, host, true); err != nil {
			logger.Println(err)
			fmt.Fprint(w, "error: "+err.Error())
			return
		}
		fmt.Fprint(w, "done")
		return
	}
}

// applyServerConfig persists one connection-method choice and everything
// that always follows from it -- optionally regenerating the TLS
// cert/server_config.json pair, marking the wizard complete, persisting
// to disk, and restarting the server on the new settings. Previously
// reimplemented three times, nearly verbatim, across the use_ip/use_ep/
// use_custom cases above.
//
// Done as two separate UpdateAPIConfig calls, not one, deliberately:
// botsetup.CreateCertCombo/CreateServerConfig read the Server fields
// back via vars.GetAPIConfig (an RLock), so they can't run from inside
// the exclusive lock UpdateAPIConfig already holds for the mutation
// without deadlocking against it. The brief window this leaves between
// "connection info updated" and "PastInitialSetup=true" is harmless --
// a crash in between just means the wizard correctly asks again, rather
// than a partial update being marked falsely complete.
func applyServerConfig(epConfig bool, port, hostOverride string, regenerateCert bool) error {
	if err := vars.UpdateAPIConfig(func(cfg *vars.Config) {
		cfg.Server.EPConfig = epConfig
		cfg.Server.Port = port
		cfg.Server.HostOverride = hostOverride
	}); err != nil {
		return fmt.Errorf("applied but failed to save to disk (will revert on restart): %w", err)
	}
	if regenerateCert {
		if err := botsetup.CreateCertCombo(); err != nil {
			return err
		}
	}
	botsetup.CreateServerConfig()
	// Choosing a connection method is the one and only step left in the
	// wizard (STT/knowledge-graph/weather are all Server Settings-only,
	// configured after setup, never here) -- so this is the single place
	// setup is considered complete.
	if err := vars.UpdateAPIConfig(func(cfg *vars.Config) {
		cfg.PastInitialSetup = true
	}); err != nil {
		return fmt.Errorf("applied but failed to save to disk (will revert on restart): %w", err)
	}
	RestartServer()
	return nil
}
