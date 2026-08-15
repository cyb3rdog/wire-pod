// Package dashboardauth gates wire-pod's local admin surface (the config
// webserver and the Lua-scripting endpoint) behind a single admin password.
//
// On first visit, whoever gets there first sets the password; after that,
// every admin/API request needs a valid session. The session is a single
// shared secret (not a per-user session table) compared via a cookie,
// persisted in APIConfig.Dashboard.SessionSecret so a login survives a
// process restart. It's rotated on a password change, which invalidates
// every existing session cookie -- including whichever browser made the
// change, which is handed a fresh cookie in the same response.
package dashboardauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "wirepod_dashboard_auth"
const cookieMaxAgeSeconds = 30 * 24 * 3600 // 30 days

var (
	sessionMu    sync.Mutex
	sessionToken string
)

func mustRandomToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing means the system RNG is broken; nothing
		// downstream of this can be trusted either.
		panic("dashboardauth: failed to generate session token: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// getSessionToken returns the value used to sign the dashboard session
// cookie. It loads APIConfig.Dashboard.SessionSecret on first use if one
// was already persisted (a prior process's setup or rotation), or
// generates and persists a new one otherwise.
func getSessionToken() string {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if sessionToken != "" {
		return sessionToken
	}
	if vars.APIConfig.Dashboard.SessionSecret != "" {
		sessionToken = vars.APIConfig.Dashboard.SessionSecret
		return sessionToken
	}
	sessionToken = mustRandomToken()
	vars.APIConfig.Dashboard.SessionSecret = sessionToken
	vars.WriteConfigToDisk()
	return sessionToken
}

// rotateSessionToken issues and persists a fresh session secret,
// invalidating every existing session cookie. Call this on a password
// change, not on first-time setup (there's no prior session to protect).
func rotateSessionToken() {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	sessionToken = mustRandomToken()
	vars.APIConfig.Dashboard.SessionSecret = sessionToken
	vars.WriteConfigToDisk()
}

func passwordSet() bool {
	return vars.APIConfig.Dashboard.PasswordHash != ""
}

func validSession(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(getSessionToken())) == 1
}

func setSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    getSessionToken(),
		Path:     "/",
		MaxAge:   cookieMaxAgeSeconds,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

// protectedPrefixes are the admin/API surfaces that need a session.
// Everything else (static webroot assets, the /ok health check, and the
// auth endpoints themselves) is left alone.
var protectedPrefixes = []string{
	"/api/",
	"/api-lua/",
	"/api-chipper/",
	"/api-sdk/",
	"/api-ble/",
	"/api-ssh/",
	"/session-certs/",
}

var publicAuthPaths = map[string]bool{
	"/api/auth/status": true,
	"/api/auth/setup":  true,
	"/api/auth/login":  true,
}

func isProtected(path string) bool {
	// Wire-pod's own first-run wizard (initial.html: escape-pod/IP mode,
	// STT language) has to run before an admin account can mean anything --
	// there's no owner to gate against yet, and its own calls (through
	// /api-chipper/ and /api/) would otherwise be locked out by the very
	// setup that's supposed to configure the server. Nothing is gated until
	// that's done.
	if !vars.APIConfig.PastInitialSetup {
		return false
	}
	if publicAuthPaths[path] {
		return false
	}
	for _, p := range protectedPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Wrap requires a valid dashboard session for every admin/API endpoint, and
// recovers a panicking handler so one bad request can't take the whole
// process down for every connected robot. Use it in place of `nil` wherever
// wire-pod currently does http.ListenAndServe(addr, nil) against the
// default mux.
func Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Println("dashboardauth: recovered panic in", r.Method, r.URL.Path, ":", rec, "\n", string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal error"}`))
			}
		}()
		if isProtected(r.URL.Path) && !validSession(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// crossSiteSetup reports whether a request to /api/auth/setup looks like it
// came from another site. Setup has no session cookie to protect it while
// the password is still unset, so without this check a malicious page could
// race the real admin to claim the account.
func crossSiteSetup(r *http.Request) bool {
	if fetchSite := r.Header.Get("Sec-Fetch-Site"); fetchSite != "" {
		return fetchSite != "same-origin" && fetchSite != "none"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// No Origin/Sec-Fetch-Site header at all is normal for same-origin
		// requests from older browsers or plain curl; don't block those.
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return !strings.EqualFold(origin, scheme+"://"+r.Host)
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var loginLimit = &loginLimiter{attempts: make(map[string][]time.Time)}

// allow permits up to 5 login attempts per minute per client IP.
func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	windowStart := now.Add(-time.Minute)
	var recent []time.Time
	for _, t := range l.attempts[ip] {
		if t.After(windowStart) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 5 {
		l.attempts[ip] = recent
		return false
	}
	l.attempts[ip] = append(recent, now)
	return true
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{msg})
	_, _ = w.Write(enc)
}

type statusResponse struct {
	// PastInitialSetup mirrors vars.APIConfig.PastInitialSetup: wire-pod's
	// own first-run wizard (initial.html) hasn't run yet, so there's
	// nothing to log into. The frontend defers to that wizard's own
	// redirect until this is true.
	PastInitialSetup bool `json:"pastInitialSetup"`
	Initialized      bool `json:"initialized"`
	Authenticated    bool `json:"authenticated"`
	// SdkEnabled mirrors vars.SDKEnabled(), so the dashboard can hide
	// bot-remote-control UI (the "Bot Settings" page, battery/connection
	// widgets) that would otherwise just fail against a disabled SDK
	// server. Fetched from the same status call the dashboard already
	// makes on every page load, rather than a dedicated endpoint.
	SdkEnabled bool `json:"sdkEnabled"`
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{
		PastInitialSetup: vars.APIConfig.PastInitialSetup,
		Initialized:      passwordSet(),
		Authenticated:    validSession(r),
		SdkEnabled:       vars.SDKEnabled(),
	})
}

type setupBody struct {
	Password string `json:"password"`
	Confirm  string `json:"confirm"`
}

// handleSetup sets the dashboard password.
//
//   - If no password exists yet, anyone who reaches this endpoint may set
//     the initial one (first-run setup).
//   - If a password already exists, changing it requires a valid session.
func handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if crossSiteSetup(r) {
		writeJSONError(w, http.StatusForbidden, "cross-site setup request rejected")
		return
	}
	isChange := passwordSet()
	if isChange && !validSession(r) {
		writeJSONError(w, http.StatusUnauthorized, "must be logged in to change the password")
		return
	}

	var body setupBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.Password) < 8 {
		writeJSONError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if body.Password != body.Confirm {
		writeJSONError(w, http.StatusBadRequest, "passwords do not match")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to set password")
		return
	}
	vars.APIConfig.Dashboard.PasswordHash = string(hash)
	if isChange {
		// Invalidates every existing session cookie, including this
		// request's own (if any) -- setSessionCookie below re-issues one
		// with the new value so the browser making the change stays
		// logged in. rotateSessionToken persists the config too, so
		// there's no separate WriteConfigToDisk call on this branch.
		rotateSessionToken()
	} else {
		vars.WriteConfigToDisk()
	}

	setSessionCookie(w, r)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type loginBody struct {
	Password string `json:"password"`
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !loginLimit.allow(clientIP(r)) {
		writeJSONError(w, http.StatusTooManyRequests, "too many login attempts, try again in a minute")
		return
	}
	if !passwordSet() {
		writeJSONError(w, http.StatusConflict, "no password has been set yet")
		return
	}

	var body loginBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(vars.APIConfig.Dashboard.PasswordHash), []byte(body.Password)) != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	setSessionCookie(w, r)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clearSessionCookie(w, r)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// RegisterRoutes registers /api/auth/{status,setup,login,logout} on the
// default mux. Call once during server startup.
func RegisterRoutes() {
	// Resolve (and, if this is the first time, persist) the session
	// secret now rather than lazily on the first request.
	getSessionToken()
	http.HandleFunc("/api/auth/status", handleStatus)
	http.HandleFunc("/api/auth/setup", handleSetup)
	http.HandleFunc("/api/auth/login", handleLogin)
	http.HandleFunc("/api/auth/logout", handleLogout)
}
