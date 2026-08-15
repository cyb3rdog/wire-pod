package wirepod_ttr

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/sashabaranov/go-openai"
)

// mockStreamingLLM serves a minimal OpenAI-compatible SSE chat completion
// stream, replying with the given content chunks in order.
func mockStreamingLLM(t *testing.T, chunks []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, c := range chunks {
			fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":\"\"}]}\n\n", c)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
}

func withTestKnowledgeConfig(t *testing.T, endpoint string) {
	t.Helper()
	origKnowledge := vars.APIConfig.Knowledge
	origBotInfoPath := vars.BotInfoPath
	origBotInfo := vars.BotInfo
	t.Cleanup(func() {
		vars.APIConfig.Knowledge = origKnowledge
		vars.BotInfoPath = origBotInfoPath
		vars.BotInfo = origBotInfo
	})
	vars.APIConfig.Knowledge.Provider = "custom"
	vars.APIConfig.Knowledge.Endpoint = endpoint
	vars.APIConfig.Knowledge.Key = "test-key"
	vars.APIConfig.Knowledge.Model = "test-model"
	vars.BotInfoPath = t.TempDir() + "/botSdkInfo.json"
	vars.BotInfo = vars.RobotInfoStore{} // no robots registered -- forces robotAvailable=false
}

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

// TestStreamingKGSimWithoutRobotStillCallsLLM guards the graceful-
// degradation fix: no robot registered for the esn (the same end state as
// a robot whose recorded IP is wrong because it's behind a reverse proxy,
// or SDK_ENABLED=false) must NOT prevent the LLM call from completing --
// the previous behavior aborted (or, for BControl/animation-loop-fed
// channels, could hang) before the LLM was ever contacted. The function
// must return the full accumulated response with no error and no robot
// connection anywhere in the path.
func TestStreamingKGSimWithoutRobotStillCallsLLM(t *testing.T) {
	logger.Init()
	srv := mockStreamingLLM(t, []string{"Hello", " from", " the", " test", " LLM."})
	defer srv.Close()
	withTestKnowledgeConfig(t, srv.URL)

	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		text, err := StreamingKGSim(nil, "unregistered-esn", "what time is it", true)
		done <- result{text, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("StreamingKGSim returned an error with no robot registered: %v", r.err)
		}
		want := "Hello from the test LLM."
		if r.text != want {
			t.Errorf("StreamingKGSim returned %q, want %q", r.text, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StreamingKGSim did not return within 5s -- likely blocked on a robot-control channel with no robot connected")
	}
}

// TestStreamingKGSimSkipsRobotWhenSDKDisabled guards the other half of
// "consider the relevant env vars": SDK_ENABLED=false must skip the
// robot-connection attempt entirely (not just tolerate its failure) and
// still let the LLM call proceed.
func TestStreamingKGSimSkipsRobotWhenSDKDisabled(t *testing.T) {
	logger.Init()
	os.Setenv("SDK_ENABLED", "false")
	t.Cleanup(func() { os.Unsetenv("SDK_ENABLED") })

	srv := mockStreamingLLM(t, []string{"Hello", " from", " the", " test", " LLM."})
	defer srv.Close()
	withTestKnowledgeConfig(t, srv.URL)

	done := make(chan error, 1)
	go func() {
		_, err := StreamingKGSim(nil, "some-esn", "what time is it", true)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("StreamingKGSim returned an error with SDK_ENABLED=false: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StreamingKGSim did not return within 5s with SDK_ENABLED=false")
	}
}

// TestStreamingKGSimHandlesResponseWithoutTerminalPunctuation guards the
// actual reported symptom: a real, non-empty LLM response that never
// contains a sentence-ending '.'/'?'/'!' (short replies like "42" or
// "yes" are common) was previously discarded entirely as "LLM returned
// no response", because fullRespSlice only ever got appended to inside
// the punctuation-triggered split logic. The full response must now be
// used as-is instead of silently lost.
func TestStreamingKGSimHandlesResponseWithoutTerminalPunctuation(t *testing.T) {
	logger.Init()
	srv := mockStreamingLLM(t, []string{"42"})
	defer srv.Close()
	withTestKnowledgeConfig(t, srv.URL)

	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		text, err := StreamingKGSim(nil, "unregistered-esn", "what is 6 times 7", true)
		done <- result{text, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("StreamingKGSim returned an error for a punctuation-less response: %v", r.err)
		}
		if r.text != "42" {
			t.Errorf("StreamingKGSim returned %q, want %q -- the response was silently discarded", r.text, "42")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StreamingKGSim did not return within 5s for a punctuation-less response")
	}
}

// TestStreamingKGSimReportsGenuinelyEmptyResponse is the flip side: a
// stream that completes with truly zero content (no choices/deltas at
// all) must still be reported as an error, not silently treated as
// success with an empty string.
func TestStreamingKGSimReportsGenuinelyEmptyResponse(t *testing.T) {
	logger.Init()
	srv := mockStreamingLLM(t, nil)
	defer srv.Close()
	withTestKnowledgeConfig(t, srv.URL)

	done := make(chan error, 1)
	go func() {
		_, err := StreamingKGSim(nil, "unregistered-esn", "say nothing", true)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("StreamingKGSim returned no error for a genuinely empty LLM response, want an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StreamingKGSim did not return within 5s for a genuinely empty response")
	}
}

// TestNewLLMClientDefaultsTogetherModelToLlama3 guards against the exact
// drift this helper was extracted to fix: StreamingKGSim and DoGetImage
// used to each carry their own copy of this provider switch, and the
// copies had diverged -- one defaulted a fresh Together config to
// "meta-llama/Llama-3-70b-chat-hf", the other still wrote the retired
// "meta-llama/Llama-2-70b-chat-hf". With both call sites now routed
// through one newLLMClient, there's exactly one default to get right.
func TestNewLLMClientDefaultsTogetherModelToLlama3(t *testing.T) {
	origKnowledge := vars.APIConfig.Knowledge
	origPath := vars.ApiConfigPath
	t.Cleanup(func() {
		vars.APIConfig.Knowledge = origKnowledge
		vars.ApiConfigPath = origPath
	})
	vars.ApiConfigPath = t.TempDir() + "/apiConfig.json"
	vars.APIConfig.Knowledge.Provider = "together"
	vars.APIConfig.Knowledge.Key = "test-key"
	vars.APIConfig.Knowledge.Model = ""

	client, endpoint := newLLMClient()

	if client == nil {
		t.Fatal("newLLMClient returned a nil client for provider \"together\"")
	}
	if endpoint != "https://api.together.xyz/v1" {
		t.Errorf("endpoint = %q, want https://api.together.xyz/v1", endpoint)
	}
	if vars.APIConfig.Knowledge.Model != "meta-llama/Llama-3-70b-chat-hf" {
		t.Errorf("default Together model = %q, want meta-llama/Llama-3-70b-chat-hf", vars.APIConfig.Knowledge.Model)
	}
}
