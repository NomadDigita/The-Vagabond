package ollama_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/ollama"
)

func TestProvider_Available(t *testing.T) {
	p := ollama.New("", "")
	if p.Available() {
		t.Fatalf("expected Available() == false with no base URL")
	}
	p2 := ollama.New("http://localhost:11434", "")
	if !p2.Available() {
		t.Fatalf("expected Available() == true with a base URL set (no API key required for Ollama)")
	}
}

func TestProvider_Complete_HappyPath(t *testing.T) {
	var capturedBody map[string]any
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"model": "llama3.1",
			"message": {"role": "assistant", "content": "hello from ollama test server"},
			"done": true,
			"done_reason": "stop",
			"prompt_eval_count": 10,
			"eval_count": 6
		}`))
	}))
	defer server.Close()

	p := ollama.New(server.URL, "llama3.1")
	resp, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_planet_governor",
		System:   "You are a test system prompt.",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello from ollama test server" {
		t.Errorf("unexpected response text: %q", resp.Text)
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 6 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}
	if capturedPath != "/api/chat" {
		t.Errorf("expected path /api/chat, got %s", capturedPath)
	}
	if streamVal, ok := capturedBody["stream"].(bool); !ok || streamVal {
		t.Errorf("expected stream:false in request body, got %v", capturedBody["stream"])
	}
}

func TestProvider_Complete_JSONModeSetsFormat(t *testing.T) {
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model": "llama3.1", "message": {"role": "assistant", "content": "{}"}, "done": true}`))
	}))
	defer server.Close()

	p := ollama.New(server.URL, "llama3.1")
	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_economy_advisor",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		JSONMode: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedBody["format"] != "json" {
		t.Errorf("expected format:json when JSONMode is true, got %v", capturedBody["format"])
	}
}

func TestProvider_Complete_NoBaseURLReturnsError(t *testing.T) {
	p := ollama.New("", "llama3.1")
	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_fleet_commander",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatalf("expected an error when no base URL is configured")
	}
}

func TestProvider_Complete_ServerUnreachable(t *testing.T) {
	p := ollama.New("http://127.0.0.1:1", "llama3.1") // port 1 should reliably refuse connections
	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_planet_governor",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatalf("expected a connection error for an unreachable Ollama host")
	}
}
