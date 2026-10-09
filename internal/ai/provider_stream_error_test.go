package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newAIStreamErrorTestService(client *http.Client) *Service {
	return &Service{
		aiHTTPClients: map[string]*http.Client{
			aiHTTPClientCacheKey("", 0): client,
		},
	}
}

func newAIStreamErrorServer(t *testing.T, event string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", event)
	}))
}

func TestMessagesStreamErrorIsReturned(t *testing.T) {
	server := newAIStreamErrorServer(t, `{"type":"error","error":{"type":"overloaded_error","message":"provider overloaded"}}`)
	defer server.Close()

	service := newAIStreamErrorTestService(server.Client())
	_, err := service.requestMessagesAIChatRound(context.Background(), "request-1", AIChatRequestPayload{SkipSystemPrompt: true}, AIProviderProfile{
		Provider: "Messages",
		BaseURL:  server.URL,
		Model:    "test-model",
	}, []AIChatRequestMessage{{Role: "user", Content: "hello"}})
	if err == nil || !strings.Contains(err.Error(), "provider overloaded") {
		t.Fatalf("expected stream error, got %v", err)
	}
}

func TestResponsesStreamErrorsAreReturned(t *testing.T) {
	tests := []struct {
		name  string
		event string
		want  string
	}{
		{
			name:  "error event",
			event: `{"type":"error","code":"server_error","message":"stream failed"}`,
			want:  "stream failed",
		},
		{
			name:  "failed response",
			event: `{"type":"response.failed","response":{"error":{"code":"server_error","message":"response failed"}}}`,
			want:  "response failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newAIStreamErrorServer(t, tt.event)
			defer server.Close()

			service := newAIStreamErrorTestService(server.Client())
			_, err := service.requestResponsesAIChatRound(context.Background(), "request-1", AIChatRequestPayload{SkipSystemPrompt: true}, AIProviderProfile{
				Provider: "Responses",
				BaseURL:  server.URL,
				Model:    "test-model",
			}, []AIChatRequestMessage{{Role: "user", Content: "hello"}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected stream error %q, got %v", tt.want, err)
			}
		})
	}
}

func TestAtomicWriteFileRemovesTempFileOnRenameFailure(t *testing.T) {
	target := t.TempDir()
	tempPath := target + ".tmp"
	if err := atomicWriteFile(target, []byte("content"), 0600); err == nil {
		t.Fatal("rename to directory should fail")
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains after failure: %v", err)
	}
}
