package initwirepod

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
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
		vars.APIConfig.PastInitialSetup = true
		botsetup.CreateServerConfig()
		vars.WriteConfigToDisk()
		RestartServer()
		fmt.Fprint(w, "done")
		return
	}
}
