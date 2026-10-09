package codexbridge

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func waitState(t *testing.T, m *Manager, want string) Status {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if status := m.Status(); status.State == want {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state = %q, want %q", m.Status().State, want)
	return Status{}
}

type mockRunner struct {
	run func(context.Context, ExecRequest, func(string) error) (ExecResult, error)
}

func (m mockRunner) Run(ctx context.Context, request ExecRequest, emit func(string) error) (ExecResult, error) {
	return m.run(ctx, request, emit)
}

func TestManagerStartsBuiltInGatewayAndStopsIt(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(exe, []byte("fake"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	m.newGateway = func(_, workingDir string) *Gateway {
		return NewGateway(mockRunner{run: func(_ context.Context, _ ExecRequest, emit func(string) error) (ExecResult, error) {
			return ExecResult{}, emit("ok")
		}}, workingDir, 2)
	}
	cfg := Config{Enabled: true, BaseURL: DefaultBaseURL, ExecutablePath: exe}
	m.Apply(cfg)
	status := waitState(t, m, "running")
	if !status.Managed || status.BaseURL == DefaultBaseURL {
		t.Fatalf("unexpected status: %+v", status)
	}
	resp, err := http.Get(status.BaseURL + "/models")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("models status = %d", resp.StatusCode)
	}
	m.Apply(cfg)
	if got := m.Status().BaseURL; got != status.BaseURL {
		t.Fatalf("idempotent apply changed URL: %s", got)
	}
	m.Close()
	if _, err := http.Get(status.BaseURL + "/models"); err == nil {
		t.Fatal("gateway still accepts requests after Close")
	}
}

func TestManagerUsesHealthyExternalGateway(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	m := NewManager()
	m.Apply(Config{Enabled: true, BaseURL: server.URL + "/v1"})
	status := waitState(t, m, "external")
	if status.Managed {
		t.Fatal("external bridge marked managed")
	}
	m.Close()
}

func TestManagerReportsMissingCodex(t *testing.T) {
	m := NewManager()
	m.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	m.homeDir = func() (string, error) { return t.TempDir(), nil }
	m.Apply(Config{Enabled: true, BaseURL: DefaultBaseURL})
	status := waitState(t, m, "not_installed")
	if status.InstallURL == "" {
		t.Fatal("missing install URL")
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	if got := NormalizeBaseURL("  "); got != DefaultBaseURL {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeBaseURL("http://localhost:5050/v1/"); got != "http://localhost:5050/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestGatewayModelsAndResponsesSSE(t *testing.T) {
	var captured ExecRequest
	runner := mockRunner{run: func(_ context.Context, request ExecRequest, emit func(string) error) (ExecResult, error) {
		captured = request
		if err := emit("hello "); err != nil {
			return ExecResult{}, err
		}
		if err := emit("world"); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{InputTokens: 12, CachedInputTokens: 3, OutputTokens: 2}, nil
	}}
	gateway := NewGateway(runner, "C:/work", 2)
	baseURL, err := gateway.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	models, err := http.Get(baseURL + "/models")
	if err != nil {
		t.Fatal(err)
	}
	models.Body.Close()
	if models.StatusCode != 200 {
		t.Fatalf("models status = %d", models.StatusCode)
	}
	body := `{"model":"gpt-5.6-sol","instructions":"be short","reasoning":{"effort":"high"},"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`
	response, err := http.Post(baseURL+"/responses", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data := new(strings.Builder)
	_, _ = io.Copy(data, response.Body)
	stream := data.String()
	for _, want := range []string{"response.output_text.delta", "hello ", "world", "response.completed", "[DONE]"} {
		if !strings.Contains(stream, want) {
			t.Fatalf("stream missing %q: %s", want, stream)
		}
	}
	if captured.Model != "gpt-5.6-sol" || captured.ReasoningEffort != "high" || !strings.Contains(captured.Prompt, "[SYSTEM]\nbe short") || !strings.Contains(captured.Prompt, "[USER]\nhi") {
		t.Fatalf("captured request = %+v", captured)
	}
}

func TestGatewayUsesResolvedSessionContext(t *testing.T) {
	localCWD := t.TempDir()
	var captured ExecRequest
	runner := mockRunner{run: func(_ context.Context, request ExecRequest, _ func(string) error) (ExecResult, error) {
		captured = request
		return ExecResult{}, nil
	}}
	gateway := NewGateway(runner, "fallback", 1)
	gateway.SetSessionContextResolver(func(sessionID string) SessionContext {
		if sessionID != "terminal-42" {
			t.Fatalf("resolver session ID = %q", sessionID)
		}
		return SessionContext{LocalWorkingDirectory: localCWD, RemoteWorkingDirectory: "/srv/project"}
	})
	baseURL, err := gateway.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/responses", strings.NewReader(`{"input":"inspect"}`))
	req.Header.Set("X-LumeTerm-Session-ID", "terminal-42")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if captured.WorkingDir != localCWD {
		t.Fatalf("Codex -C directory = %q, want %q", captured.WorkingDir, localCWD)
	}
	if !strings.Contains(captured.Prompt, `Remote working directory (quoted): "/srv/project"`) || !strings.Contains(captured.Prompt, "[USER]\ninspect") {
		t.Fatalf("resolved prompt context missing: %q", captured.Prompt)
	}
	t.Logf("resolved Codex ExecRequest working_dir=%q", captured.WorkingDir)
}

func TestGatewaySerializesSameSession(t *testing.T) {
	var active, maximum atomic.Int32
	release := make(chan struct{})
	runner := mockRunner{run: func(ctx context.Context, _ ExecRequest, _ func(string) error) (ExecResult, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			old := maximum.Load()
			if current <= old || maximum.CompareAndSwap(old, current) {
				break
			}
		}
		select {
		case <-release:
			return ExecResult{}, nil
		case <-ctx.Done():
			return ExecResult{}, ctx.Err()
		}
	}}
	gateway := NewGateway(runner, ".", 2)
	baseURL, err := gateway.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	client := &http.Client{}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("POST", baseURL+"/responses", strings.NewReader(`{"input":"hi"}`))
			req.Header.Set("X-LumeTerm-Conversation-ID", "same")
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	time.Sleep(100 * time.Millisecond)
	if maximum.Load() != 1 {
		t.Fatalf("same-session concurrency = %d", maximum.Load())
	}
	close(release)
	wg.Wait()
}

func TestGatewayCanceledRequestStopsWaitingForSession(t *testing.T) {
	gateway := NewGateway(mockRunner{}, ".", 2)
	unlockFirst, ok := gateway.lockSession(context.Background(), "same")
	if !ok {
		t.Fatal("first lock was not acquired")
	}
	ctx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan bool, 1)
	go func() {
		_, acquired := gateway.lockSession(ctx, "same")
		secondDone <- acquired
	}()
	deadline := time.Now().Add(time.Second)
	for {
		gateway.sessionsMu.Lock()
		entry := gateway.sessions["same"]
		refs := 0
		if entry != nil {
			refs = entry.refs
		}
		gateway.sessionsMu.Unlock()
		if refs == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second request did not begin waiting on the session lock")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case acquired := <-secondDone:
		if acquired {
			t.Fatal("canceled waiter acquired the session lock")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request remained blocked on the session lock")
	}
	unlockFirst()
	gateway.sessionsMu.Lock()
	remaining := len(gateway.sessions)
	gateway.sessionsMu.Unlock()
	if remaining != 0 {
		t.Fatalf("session lock entries remaining = %d", remaining)
	}
}

func TestRealCodexGatewaySmoke(t *testing.T) {
	executable := strings.TrimSpace(os.Getenv("CODEX_INTEGRATION_EXE"))
	if executable == "" {
		t.Skip("set CODEX_INTEGRATION_EXE to run the real Codex gateway smoke test")
	}
	gateway := NewGateway(nativeExecRunner{executable: executable, readOnly: true}, ".", 1)
	sessionCWD := t.TempDir()
	gateway.SetSessionContextResolver(func(sessionID string) SessionContext {
		return SessionContext{LocalWorkingDirectory: sessionCWD}
	})
	baseURL, err := gateway.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	body := `{"model":"gpt-5.6-sol","reasoning":{"effort":"medium"},"input":"Only output LUME_HTTP_GATEWAY_OK. Do not use tools."}`
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/responses", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-LumeTerm-Session-ID", "integration-terminal")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data := new(strings.Builder)
	_, _ = io.Copy(data, response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(data.String(), "LUME_HTTP_GATEWAY_OK") || !strings.Contains(data.String(), "response.completed") {
		t.Fatalf("unexpected gateway response (%d): %s", response.StatusCode, data.String())
	}
	t.Logf("real gateway session working_dir=%q SSE: %s", sessionCWD, data.String())
}
