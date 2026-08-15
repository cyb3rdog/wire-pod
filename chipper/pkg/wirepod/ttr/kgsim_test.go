package wirepod_ttr

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/sashabaranov/go-openai"
)

// TestLogLLMErrorExtractsRequestErrorBody guards the actual reported
// symptom: a custom/self-hosted LLM endpoint behind a reverse proxy
// returning a 502 with a non-JSON (typically HTML) body. go-openai wraps
// that as a *RequestError; logLLMError must surface its real status code
// and body, not just whatever generic string the error happened to
// stringify to, and it must reach both log sinks (LogTray -- docker logs
// / /api/get_debug_logs -- and LogUI -- dashboard "recent activity"),
// unlike the stdlib log.Printf call this replaced, which reached neither.
func TestLogLLMErrorExtractsRequestErrorBody(t *testing.T) {
	logger.Init()

	reqErr := &openai.RequestError{
		HTTPStatusCode: 502,
		HTTPStatus:     "502 Bad Gateway",
		Body:           []byte("<html><body>502 Bad Gateway</body></html>"),
	}

	logLLMError("creating chat completion stream", "https://llm.example.com/v1", "my-model", 3500*time.Millisecond, reqErr)

	tray := logger.GetLogTrayList()
	if !strings.Contains(tray, "502") {
		t.Error("log tray missing status code 502")
	}
	if !strings.Contains(tray, "llm.example.com") {
		t.Error("log tray missing the actual endpoint that was called")
	}
	if !strings.Contains(tray, "my-model") {
		t.Error("log tray missing the model that was requested")
	}
	if !strings.Contains(tray, "3.5s") {
		t.Error("log tray missing elapsed time before the call failed")
	}
	if !strings.Contains(tray, "502 Bad Gateway") {
		t.Error("log tray missing the raw response body, want the HTML the proxy actually returned")
	}

	ui := logger.GetLogList()
	if !strings.Contains(ui, "502") {
		t.Error("dashboard log (LogUI) missing status code 502 -- the old log.Printf call never reached this sink at all")
	}
}

// TestLogLLMErrorExtractsAPIErrorMessage covers the other common shape:
// a real OpenAI-style JSON error body, which go-openai parses into an
// *APIError instead of a *RequestError.
func TestLogLLMErrorExtractsAPIErrorMessage(t *testing.T) {
	logger.Init()

	apiErr := &openai.APIError{
		HTTPStatusCode: 401,
		HTTPStatus:     "401 Unauthorized",
		Message:        "Incorrect API key provided",
	}

	logLLMError("creating chat completion stream", "https://api.openai.com/v1", "gpt-4o-mini", 200*time.Millisecond, apiErr)

	tray := logger.GetLogTrayList()
	if !strings.Contains(tray, "401") {
		t.Error("log tray missing status code 401")
	}
	if !strings.Contains(tray, "Incorrect API key provided") {
		t.Error("log tray missing the API's actual error message")
	}
}

// TestLogLLMErrorFallsBackToErrorString guards the third shape: an error
// that's neither a RequestError nor an APIError (e.g. a network-level
// failure such as a connection refused/timeout) -- must still log
// something useful rather than silently doing nothing.
func TestLogLLMErrorFallsBackToErrorString(t *testing.T) {
	logger.Init()

	plainErr := errors.New("dial tcp: connection refused")

	logLLMError("creating chat completion stream", "https://llm.example.com/v1", "my-model", time.Second, plainErr)

	tray := logger.GetLogTrayList()
	if !strings.Contains(tray, "connection refused") {
		t.Error("log tray missing the underlying error text for a non-HTTP failure")
	}
}
