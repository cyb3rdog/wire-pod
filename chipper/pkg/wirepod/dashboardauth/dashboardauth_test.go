package dashboardauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

func resetPassword(t *testing.T) {
	t.Helper()
	vars.ApiConfigPath = t.TempDir() + "/apiConfig.json"
	vars.APIConfig.Dashboard.PasswordHash = ""
}

func TestSetupThenLoginFlow(t *testing.T) {
	resetPassword(t)

	// Before setup, status reports uninitialized.
	rec := httptest.NewRecorder()
	handleStatus(rec, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))
	var status statusResponse
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Initialized {
		t.Fatal("expected Initialized=false before setup")
	}

	// Setup with a fresh password.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"password":"correct-horse","confirm":"correct-horse"}`))
	req.Header.Set("Origin", "http://"+req.Host)
	handleSetup(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !passwordSet() {
		t.Fatal("expected password to be set after setup")
	}

	// A second setup attempt without a session should be rejected.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"password":"other","confirm":"other"}`))
	req.Header.Set("Origin", "http://"+req.Host)
	handleSetup(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 re-setup without session, got %d", rec.Code)
	}

	// Wrong password fails.
	rec = httptest.NewRecorder()
	handleLogin(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"wrong"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password, got %d", rec.Code)
	}

	// Correct password succeeds and sets a session cookie.
	rec = httptest.NewRecorder()
	handleLogin(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"correct-horse"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for correct password, got %d: %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieName {
		t.Fatalf("expected a %s cookie to be set, got %+v", cookieName, cookies)
	}

	// That cookie authenticates a protected request.
	req = httptest.NewRequest(http.MethodGet, "/api/get_config", nil)
	req.AddCookie(cookies[0])
	if !validSession(req) {
		t.Fatal("expected session cookie to validate")
	}
}

func TestWrapBlocksProtectedPathsWithoutSession(t *testing.T) {
	resetPassword(t)
	vars.APIConfig.Dashboard.PasswordHash = "$2a$10$abcdefghijklmnopqrstuv" // any non-empty hash

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Wrap(next)

	cases := []struct {
		path       string
		wantStatus int
	}{
		{"/api/get_config", http.StatusUnauthorized},
		{"/api-lua/run_script", http.StatusUnauthorized},
		{"/api-chipper/restart", http.StatusUnauthorized},
		{"/session-certs/00e20145", http.StatusUnauthorized},
		{"/api/auth/status", http.StatusOK},
		{"/api/auth/login", http.StatusOK},
		{"/ok", http.StatusOK},
		{"/index.html", http.StatusOK},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.wantStatus {
			t.Errorf("%s: expected %d, got %d", c.path, c.wantStatus, rec.Code)
		}
	}
}

func TestLoginRateLimiter(t *testing.T) {
	l := &loginLimiter{attempts: make(map[string][]time.Time)}
	for i := 0; i < 5; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("attempt %d: expected allow", i)
		}
	}
	if l.allow("1.2.3.4") {
		t.Fatal("6th attempt within a minute should be blocked")
	}
	// A different IP is not affected by the first one's limit.
	if !l.allow("5.6.7.8") {
		t.Fatal("expected a different IP to be unaffected")
	}
}
