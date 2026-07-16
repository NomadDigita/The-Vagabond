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

	text := placeholderJSON(req.Feature)
	if text == "" {
		// Unknown feature or non-JSON caller: fall back to a plain,
		// clearly-labeled echo rather than guessing at a JSON shape
		// this provider doesn't know.
		text = fmt.Sprintf(
			"[mock-ai] No live AI provider is configured, so this is a placeholder response for feature %q. "+
				"Set an API key for a real provider (see /ai_status) to get real recommendations. Echo of your input: %q",
			req.Feature, truncate(last, 200),
		)
	}

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

// placeholderJSON returns a valid, well-formed JSON object matching
// the structured shape each Phase B-J feature parses, so a request
// made with JSONMode still renders through that feature's real
// formatted template (FormatForTelegram) instead of falling back to a
// raw-text dump. Every field is explicit that it's a placeholder — no
// output here is presented as if it were a genuine recommendation.
//
// The field names mirror the `json:"..."` tags in
// internal/game/governor, internal/game/fleetcommander, and
// internal/game/econadvisor. This package intentionally does NOT
// import those packages (mock must stay a pure internal/ai provider
// with no game dependency) — the shapes are duplicated here as plain
// string literals, which is an acceptable small duplication for that
// isolation. If a phase's JSON shape changes, its own tests will catch
// a mismatch (ParseRecommendation will fail and fall back to raw
// text) — update the corresponding case below to match.
func placeholderJSON(feature string) string {
	switch feature {
	case "ai_planet_governor":
		return `{
			"summary": "🔧 PLACEHOLDER (no live AI configured) — set an API key to get a real building/upgrade analysis for this base.",
			"priority_actions": [
				{"action": "configure_provider", "target": "server environment", "reason": "This is mock output. See /ai_status for setup instructions."}
			],
			"storage_warning": "Not evaluated — placeholder mode.",
			"expected_impact": "Not evaluated — placeholder mode."
		}`
	case "ai_fleet_commander":
		return `{
			"recommendation": "wait",
			"confidence": "n/a",
			"reasoning": "🔧 PLACEHOLDER (no live AI configured) — set an API key to get a real fleet-vs-target analysis. 'wait' is shown here only as a safe default, not a genuine recommendation.",
			"risk_assessment": "Not evaluated — placeholder mode.",
			"suggested_split": ""
		}`
	case "ai_economy_advisor":
		return `{
			"summary": "🔧 PLACEHOLDER (no live AI configured) — set an API key to get a real ROI/market analysis for this base.",
			"top_roi_actions": [
				{"action": "configure_provider", "target": "server environment", "reason": "This is mock output. See /ai_status for setup instructions.", "expected_gain": "n/a"}
			],
			"bottlenecks": "Not evaluated — placeholder mode.",
			"market_timing": "Not evaluated — placeholder mode.",
			"trading_advice": "Not evaluated — placeholder mode."
		}`
	case "ai_research_planner":
		return `{
			"summary": "🔧 PLACEHOLDER (no live AI configured) — set an API key to get a real research-order analysis for this base.",
			"goal_used": "n/a",
			"recommended_order": [
				{"node": "configure_provider", "target_level": 0, "reason": "This is mock output. See /ai_status for setup instructions.", "core_cost": 0, "expected_gain": "n/a"}
			],
			"cores_needed": 0,
			"cores_available": 0,
			"notes": "Not evaluated — placeholder mode."
		}`
	default:
		return ""
	}
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
