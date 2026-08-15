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
		_, err := strconv.Atoi(port)
		if err != nil {
			fmt.Fprint(w, "error: port is invalid")
			return
		}
		vars.APIConfig.Server.EPConfig = false
		vars.APIConfig.Server.Port = port
		vars.APIConfig.Server.HostOverride = ""
		err = botsetup.CreateCertCombo()
		botsetup.CreateServerConfig()
		if err != nil {
			logger.Println(err)
			fmt.Fprint(w, "error: "+err.Error())
			return
		}
		vars.APIConfig.PastInitialSetup = true
		vars.WriteConfigToDisk()
		RestartServer()
		fmt.Fprint(w, "done")
		return
	case r.URL.Path == "/api-chipper/use_ep":
		vars.APIConfig.Server.EPConfig = true
		vars.APIConfig.Server.Port = "443"
		vars.APIConfig.Server.HostOverride = ""
		vars.APIConfig.PastInitialSetup = true
		botsetup.CreateServerConfig()
		vars.WriteConfigToDisk()
		RestartServer()
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
		vars.APIConfig.Server.EPConfig = false
		vars.APIConfig.Server.Port = port
		vars.APIConfig.Server.HostOverride = host
		err := botsetup.CreateCertCombo()
		botsetup.CreateServerConfig()
		if err != nil {
			logger.Println(err)
			fmt.Fprint(w, "error: "+err.Error())
			return
		}
		vars.APIConfig.PastInitialSetup = true
		vars.WriteConfigToDisk()
		RestartServer()
		fmt.Fprint(w, "done")
		return
	}
}
