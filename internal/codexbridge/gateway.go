package codexbridge

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var builtinModels = []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-6-sol", "gpt-6-astra", "gpt-6-luna", "gpt-5.5"}

type ExecRequest struct{ Model, ReasoningEffort, Prompt, WorkingDir string }
type ExecResult struct{ InputTokens, CachedInputTokens, OutputTokens int }
type ExecRunner interface {
	Run(context.Context, ExecRequest, func(string) error) (ExecResult, error)
}

// SessionContext is resolved by the host application from its authoritative
// terminal-session registry. A remote directory is context for the model only:
// it must never be passed to the local Codex process as -C.
type SessionContext struct {
	LocalWorkingDirectory  string
	RemoteWorkingDirectory string
}

type SessionContextResolver func(sessionID string) SessionContext

type nativeExecRunner struct {
	executable string
	readOnly   bool
}
type codexJSONEvent struct {
	Type string `json:"type"`
	Item struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
	Usage struct {
		InputTokens       int `json:"input_tokens"`
		CachedInputTokens int `json:"cached_input_tokens"`
		OutputTokens      int `json:"output_tokens"`
	} `json:"usage"`
}

func (r nativeExecRunner) Run(ctx context.Context, request ExecRequest, onText func(string) error) (ExecResult, error) {
	args := []string{"exec", "-C", request.WorkingDir, "-m", request.Model, "-c",
		fmt.Sprintf("model_reasoning_effort=%q", request.ReasoningEffort)}
	if r.readOnly {
		args = append(args, "--sandbox", "read-only")
	} else {
		args = append(args, "--approve-for-me")
	}
	log.Printf("codex gateway: starting exec model=%s working_dir=%q", request.Model, request.WorkingDir)
	// Terminal sessions commonly point at directories that are not Git
	// repositories. The host has already validated -C as an existing local
	// directory, so do not let the CLI's repository guard reject it.
	args = append(args, "--skip-git-repo-check", "--ephemeral", "--json", "-")
	cmd := exec.Command(r.executable, args...)
	configureCommand(cmd)
	cmd.Stdin = strings.NewReader(request.Prompt)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ExecResult{}, fmt.Errorf("open Codex stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ExecResult{}, fmt.Errorf("open Codex stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return ExecResult{}, fmt.Errorf("start Codex: %w", err)
	}
	var stderrText strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			if stderrText.Len() < 32*1024 {
				stderrText.WriteString(s.Text())
				stderrText.WriteByte('\n')
			}
		}
	}()
	cancelDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killProcessTree(cmd)
		case <-cancelDone:
		}
	}()
	var result ExecResult
	s := bufio.NewScanner(stdout)
	s.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var parseErr error
	for s.Scan() {
		var event codexJSONEvent
		if err := json.Unmarshal(s.Bytes(), &event); err != nil {
			log.Printf("codex gateway: ignoring malformed JSONL event: %v", err)
			continue
		}
		switch event.Type {
		case "item.completed":
			if event.Item.Type == "agent_message" && event.Item.Text != "" {
				if err := onText(event.Item.Text); err != nil {
					parseErr = err
					killProcessTree(cmd)
				}
			}
		case "turn.completed":
			result = ExecResult{event.Usage.InputTokens, event.Usage.CachedInputTokens, event.Usage.OutputTokens}
		}
		if parseErr != nil {
			break
		}
	}
	if err := s.Err(); err != nil && parseErr == nil {
		parseErr = fmt.Errorf("read Codex output: %w", err)
	}
	waitErr := cmd.Wait()
	close(cancelDone)
	<-stderrDone
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if parseErr != nil {
		return result, parseErr
	}
	if waitErr != nil {
		detail := strings.TrimSpace(stderrText.String())
		if detail == "" {
			detail = waitErr.Error()
		}
		return result, fmt.Errorf("Codex exec failed: %s", detail)
	}
	return result, nil
}

type Gateway struct {
	runner     ExecRunner
	workingDir string
	server     *http.Server
	semaphore  chan struct{}
	sessionsMu sync.Mutex
	sessions   map[string]*sessionLock
	resolverMu sync.RWMutex
	resolver   SessionContextResolver
}
type sessionLock struct {
	gate chan struct{}
	refs int
}

func NewGateway(runner ExecRunner, workingDir string, maxConcurrent int) *Gateway {
	if maxConcurrent < 1 {
		maxConcurrent = 2
	}
	if strings.TrimSpace(workingDir) == "" {
		workingDir, _ = os.Getwd()
	}
	return &Gateway{runner: runner, workingDir: workingDir, semaphore: make(chan struct{}, maxConcurrent), sessions: make(map[string]*sessionLock)}
}

func (g *Gateway) SetSessionContextResolver(resolve SessionContextResolver) {
	if g == nil {
		return
	}
	g.resolverMu.Lock()
	g.resolver = resolve
	g.resolverMu.Unlock()
}

func (g *Gateway) sessionContext(sessionID string) SessionContext {
	g.resolverMu.RLock()
	resolve := g.resolver
	g.resolverMu.RUnlock()
	if resolve == nil || strings.TrimSpace(sessionID) == "" {
		return SessionContext{}
	}
	return resolve(sessionID)
}

func (g *Gateway) Start() (string, error) {
	if g == nil || g.runner == nil {
		return "", errors.New("Codex gateway runner is unavailable")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen for Codex gateway: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", g.handleModels)
	mux.HandleFunc("/v1/responses", g.handleResponses)
	g.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := g.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("codex gateway: HTTP server failed: %v", err)
		}
	}()
	return "http://" + listener.Addr().String() + "/v1", nil
}

func (g *Gateway) Close(ctx context.Context) error {
	if g == nil || g.server == nil {
		return nil
	}
	if err := g.server.Shutdown(ctx); err != nil {
		_ = g.server.Close()
		return err
	}
	return nil
}

func (g *Gateway) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, 405, "method_not_allowed", "only GET is supported")
		return
	}
	models := make([]map[string]any, 0, len(builtinModels))
	for _, id := range builtinModels {
		models = append(models, map[string]any{"id": id, "object": "model", "owned_by": "codex"})
	}
	writeJSON(w, 200, map[string]any{"object": "list", "data": models})
}

type responsesRequest struct {
	Model          string          `json:"model"`
	Input          json.RawMessage `json:"input"`
	Instructions   string          `json:"instructions"`
	PromptCacheKey string          `json:"prompt_cache_key"`
	Reasoning      struct {
		Effort string `json:"effort"`
	} `json:"reasoning"`
}

func (g *Gateway) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, 405, "method_not_allowed", "only POST is supported")
		return
	}
	var request responsesRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024*1024)).Decode(&request); err != nil {
		writeJSONError(w, 400, "invalid_request_error", "invalid Responses request: "+err.Error())
		return
	}
	prompt, err := buildPrompt(request.Instructions, request.Input, 20)
	if err != nil {
		writeJSONError(w, 400, "invalid_request_error", err.Error())
		return
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = builtinModels[0]
	}
	effort := strings.TrimSpace(request.Reasoning.Effort)
	if effort == "" {
		effort = "medium"
	}
	sessionID := strings.TrimSpace(r.Header.Get("X-LumeTerm-Conversation-ID"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(request.PromptCacheKey)
	}
	unlock, locked := g.lockSession(r.Context(), sessionID)
	if !locked {
		return
	}
	defer unlock()
	terminalID := strings.TrimSpace(r.Header.Get("X-LumeTerm-Session-ID"))
	sessionContext := g.sessionContext(terminalID)
	workingDir := g.workingDir
	if candidate := strings.TrimSpace(sessionContext.LocalWorkingDirectory); candidate != "" {
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			workingDir = candidate
		} else {
			log.Printf("codex gateway: ignoring unavailable local session cwd session=%q cwd=%q err=%v", terminalID, candidate, statErr)
		}
	}
	if remoteCWD := strings.TrimSpace(sessionContext.RemoteWorkingDirectory); remoteCWD != "" {
		prompt = fmt.Sprintf("[LUMETERM TERMINAL CONTEXT]\nSession ID: %s\nRemote working directory (quoted): %q\nThe Codex process runs locally; do not treat this remote path as its local working directory.\n\n%s", terminalID, remoteCWD, prompt)
	}
	select {
	case g.semaphore <- struct{}{}:
		defer func() { <-g.semaphore }()
	case <-r.Context().Done():
		return
	}
	responseID := "resp_" + randomID()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, 500, "server_error", "streaming is unsupported")
		return
	}
	_ = writeSSE(w, map[string]any{"type": "response.created", "response": map[string]any{"id": responseID, "status": "in_progress"}})
	flusher.Flush()
	var output strings.Builder
	execCtx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()
	result, runErr := g.runner.Run(execCtx, ExecRequest{model, effort, prompt, workingDir}, func(delta string) error {
		output.WriteString(delta)
		if err := writeSSE(w, map[string]any{"type": "response.output_text.delta", "delta": delta}); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if runErr != nil {
		if r.Context().Err() == nil {
			log.Printf("codex gateway: request failed: %v", runErr)
			_ = writeSSE(w, map[string]any{"type": "error", "code": "codex_exec_failed", "message": runErr.Error()})
			flusher.Flush()
		}
		return
	}
	_ = writeSSE(w, map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id": responseID, "status": "completed", "output_text": output.String(),
			"usage": map[string]any{"input_tokens": result.InputTokens, "output_tokens": result.OutputTokens,
				"input_tokens_details": map[string]any{"cached_tokens": result.CachedInputTokens}},
		},
	})
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (g *Gateway) lockSession(ctx context.Context, id string) (func(), bool) {
	if id == "" {
		return func() {}, true
	}
	g.sessionsMu.Lock()
	entry := g.sessions[id]
	if entry == nil {
		entry = &sessionLock{gate: make(chan struct{}, 1)}
		entry.gate <- struct{}{}
		g.sessions[id] = entry
	}
	entry.refs++
	g.sessionsMu.Unlock()
	select {
	case <-entry.gate:
		return func() {
			entry.gate <- struct{}{}
			g.releaseSessionLock(id, entry)
		}, true
	case <-ctx.Done():
		g.releaseSessionLock(id, entry)
		return nil, false
	}
}

func (g *Gateway) releaseSessionLock(id string, entry *sessionLock) {
	g.sessionsMu.Lock()
	defer g.sessionsMu.Unlock()
	entry.refs--
	if entry.refs == 0 && g.sessions[id] == entry {
		delete(g.sessions, id)
	}
}

func buildPrompt(instructions string, rawInput json.RawMessage, limit int) (string, error) {
	var input any
	if len(rawInput) == 0 || string(rawInput) == "null" {
		return "", errors.New("input is required")
	}
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	var messages []any
	switch value := input.(type) {
	case string:
		messages = []any{map[string]any{"role": "user", "content": value}}
	case []any:
		messages = value
	default:
		return "", errors.New("input must be a string or message array")
	}
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	var prompt strings.Builder
	if text := strings.TrimSpace(instructions); text != "" {
		prompt.WriteString("[SYSTEM]\n")
		prompt.WriteString(text)
		prompt.WriteString("\n\n")
	}
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		text := extractText(message["content"])
		if text == "" {
			text, _ = message["text"].(string)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if strings.TrimSpace(role) == "" {
			role = "user"
		}
		prompt.WriteString("[" + strings.ToUpper(role) + "]\n" + text + "\n\n")
	}
	if prompt.Len() == 0 {
		return "", errors.New("input contains no text")
	}
	return strings.TrimSpace(prompt.String()), nil
}

func extractText(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		var parts []string
		for _, raw := range value {
			part, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			typeName, _ := part["type"].(string)
			if typeName == "input_text" || typeName == "output_text" || typeName == "text" {
				if text, _ := part["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func writeSSE(w http.ResponseWriter, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"type": code, "code": code, "message": message}})
}
func randomID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}
