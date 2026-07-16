package gemini_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/gemini"
)

func TestProvider_Available(t *testing.T) {
	p := gemini.New("", "")
	if p.Available() {
		t.Fatalf("expected Available() == false with no API key")
	}
	p2 := gemini.New("test-key", "")
	if !p2.Available() {
		t.Fatalf("expected Available() == true with an API key set")
	}
	if p2.DefaultModel != "gemini-3.5-flash" {
		t.Errorf("expected default model gemini-3.5-flash (2.0-flash was shut down 2026-06-01), got %q", p2.DefaultModel)
	}
}

// TestProvider_Complete_HappyPath exercises the real Gemini wire
// format end-to-end against a local server standing in for
// generativelanguage.googleapis.com, using the provider's overridable
// BaseURL field.
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
			"candidates": [{
				"content": {"role": "model", "parts": [{"text": "hello from gemini test server"}]},
				"finishReason": "STOP"
			}],
			"usageMetadata": {"promptTokenCount": 8, "candidatesTokenCount": 5}
		}`))
	}))
	defer server.Close()

	p := gemini.New("test-key", "gemini-3.5-flash")
	p.BaseURL = server.URL

	resp, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_planet_governor",
		System:   "You are a test system prompt.",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}, {Role: ai.RoleAssistant, Content: "hi there"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "hello from gemini test server" {
		t.Errorf("unexpected response text: %q", resp.Text)
	}
	if resp.Usage.InputTokens != 8 || resp.Usage.OutputTokens != 5 {
		t.Errorf("unexpected usage: %+v", resp.Usage)
	}
	if !strings.Contains(capturedPath, "gemini-3.5-flash") {
		t.Errorf("expected request path to reference the model name, got %q", capturedPath)
	}

	// Verify Gemini-specific role mapping: RoleAssistant must become
	// "model", not "assistant" — this is the whole reason Gemini has
	// its own provider instead of reusing openaicompat.
	contents, ok := capturedBody["contents"].([]any)
	if !ok || len(contents) != 2 {
		t.Fatalf("expected 2 content entries, got: %+v", capturedBody["contents"])
	}
	secondContent := contents[1].(map[string]any)
	if secondContent["role"] != "model" {
		t.Errorf("expected assistant role to map to 'model', got %v", secondContent["role"])
	}

	// Verify systemInstruction was set as its own top-level field
	// (not folded into contents), per Gemini's actual API shape.
	sysInstr, ok := capturedBody["systemInstruction"].(map[string]any)
	if !ok {
		t.Fatalf("expected systemInstruction field in request body, got: %+v", capturedBody)
	}
	parts, _ := sysInstr["parts"].([]any)
	if len(parts) == 0 {
		t.Fatalf("expected systemInstruction to have at least one part")
	}
}

func TestProvider_Complete_JSONModeSetsResponseMimeType(t *testing.T) {
	var capturedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates": [{"content": {"role": "model", "parts": [{"text": "{}"}]}, "finishReason": "STOP"}], "usageMetadata": {"promptTokenCount": 1, "candidatesTokenCount": 1}}`))
	}))
	defer server.Close()

	p := gemini.New("test-key", "gemini-3.5-flash")
	p.BaseURL = server.URL

	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_economy_advisor",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
		JSONMode: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	genConfig, ok := capturedBody["generationConfig"].(map[string]any)
	if !ok {
		t.Fatalf("expected generationConfig in request body")
	}
	if genConfig["responseMimeType"] != "application/json" {
		t.Errorf("expected responseMimeType application/json when JSONMode is true, got %v", genConfig["responseMimeType"])
	}
}

func TestProvider_Complete_NoAPIKeyReturnsError(t *testing.T) {
	p := gemini.New("", "gemini-3.5-flash")
	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_planet_governor",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatalf("expected an error when no API key is configured")
	}
}

func TestProvider_Complete_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": {"message": "invalid request", "status": "INVALID_ARGUMENT"}}`))
	}))
	defer server.Close()

	p := gemini.New("test-key", "gemini-3.5-flash")
	p.BaseURL = server.URL

	_, err := p.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_fleet_commander",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatalf("expected an error for a 400 response")
	}
}
