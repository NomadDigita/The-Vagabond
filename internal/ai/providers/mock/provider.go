// Package mock is a zero-dependency AI Provider used as the default
// fallback so The Vagabond always boots and every AI-driven feature
// degrades gracefully instead of erroring when no real provider is
// configured. It never makes network calls.
package mock

import (
	"context"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// Provider implements ai.Provider with canned, deterministic replies.
// It is always Available(), so it belongs last in every fallback chain.
type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string    { return "mock" }
func (p *Provider) Available() bool { return true }

func (p *Provider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	var last string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == ai.RoleUser {
			last = req.Messages[i].Content
			break
		}
	}

	text := fmt.Sprintf(
		"[mock-ai] No live AI provider is configured, so this is a placeholder response for feature %q. "+
			"Set ANTHROPIC_API_KEY (or register another provider) to get real recommendations. Echo of your input: %q",
		req.Feature, truncate(last, 200),
	)

	return &ai.CompletionResponse{
		Text:       text,
		Model:      "mock-1",
		StopReason: "end_turn",
		Usage: ai.Usage{
			InputTokens:  estimateTokens(req.System) + estimateTokens(last),
			OutputTokens: estimateTokens(text),
		},
	}, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// estimateTokens is a rough (chars/4) heuristic used only so the mock
// provider exercises the cost-accounting path in tests and local dev;
// it is never used for real billing decisions.
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return len(s)/4 + 1
}
