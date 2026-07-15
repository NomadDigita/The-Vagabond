package openaicompat_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/openaicompat"
)

func TestProvider_Available(t *testing.T) {
	p := openaicompat.New("openai", "https://api.openai.com/v1", "", "gpt-4o-mini", true)
	if p.Available() {
		t.Fatalf("expected Available() == false with no API key")
	}
	p2 := openaicompat.New("openai", "https://api.openai.com/v1", "sk-test", "gpt-4o-mini", true)
	if !p2.Available() {
		t.Fatalf("expected Available() == true with an API key set")
	}
}

// TestProvider_Complete_HappyPath spins up a local HTTP server that
// mimics the OpenAI Chat Completions response shape and verifies the
// provider both sends a well-formed request and correctly parses the
// response — this is a real end-to-end exercise of the wire format,
// not an assumption that the JSON shapes line up.
func TestProvider_Complete_HappyPath(t *testing.T) {
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer sk-test" {
			t.Errorf("expected Authorization header 'Bearer sk-test', got %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "hello from test server"}, "finish_reason": "stop"}],
			"model": "gpt-4o-mini",
			"usage": {"prompt_tokens": 12, "completion_tokens": 4}
		}`))
	}))
	defer server.Close()

	p := openaicompat.New("openai", server.URL, "sk-test", "gpt-4o-mini", true)
	resp, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:   "ai_planet_governor",
		System:    "You are a test system prompt.",
		Messages:  []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello from test server" {
		t.Errorf("unexpected response text: %q", resp.Text)
	}
	if resp.Usage.InputTokens != 12 || resp.Usage.OutputTokens != 4 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}

	// Verify the request we actually sent had the right shape.
	msgs, ok := capturedBody["messages"].([]any)
	if !ok || len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system + user) in request body, got: %+v", capturedBody["messages"])
	}
	firstMsg := msgs[0].(map[string]any)
	if firstMsg["role"] != "system" {
		t.Errorf("expected first message role 'system', got %v", firstMsg["role"])
	}
}

// TestProvider_Complete_JSONModeWithoutSystemPrompt verifies the
// JSON-mode instruction still gets attached even when the caller
// passed no System prompt at all — this exercises a real edge case
// (not just the common path where System is always set).
func TestProvider_Complete_JSONModeWithoutSystemPrompt(t *testing.T) {
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices": [{"message": {"role": "assistant", "content": "{}"}, "finish_reason": "stop"}], "model": "gpt-4o-mini", "usage": {"prompt_tokens": 1, "completion_tokens": 1}}`))
	}))
	defer server.Close()

	p := openaicompat.New("openai", server.URL, "sk-test", "gpt-4o-mini", true)
	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_fleet_commander",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		JSONMode: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs, _ := capturedBody["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatalf("expected at least one message in request body")
	}
	firstMsg := msgs[0].(map[string]any)
	// With no System prompt supplied, the JSON-mode instruction should
	// still have been added as its own system message rather than
	// silently dropped.
	if firstMsg["role"] != "system" {
		t.Errorf("expected a system message to carry the JSON-mode instruction even with empty System, got first message role %v (body: %+v)", firstMsg["role"], capturedBody)
	}
}

func TestProvider_Complete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid api key", "type": "authentication_error"}}`))
	}))
	defer server.Close()

	p := openaicompat.New("openai", server.URL, "sk-bad", "gpt-4o-mini", true)
	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_economy_advisor",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatalf("expected an error for a 401 response, got nil")
	}
}
