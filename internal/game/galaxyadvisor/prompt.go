// Package galaxyadvisor implements Phase H of the AI Systems Roadmap:
// the AI Dynamic Galaxy advisor. Like every other Phase B-G package,
// this is the seam where game state meets ai.Service; internal/ai
// itself stays game-agnostic per ADR-001.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in advisor.go.
//
// Unlike every prior package (which reasons about one player's base,
// fleet, economy, research, combat record, or clan), this one is
// genuinely cross-cutting, matching how §3 flagged Phase H: it reasons
// about the shared, per-continent environmental state — world events
// rolled independently for Africa/Europe/Asia/Americas by
// internal/engine/world.WeatherEngine (see migrations/025, SpaceHunt
// Phase 7 item 12) — both for the player's own continent specifically
// and across the wider galaxy, so a player can also reason about
// whether now's a good time to march into (or avoid) another
// continent.
//
// Scope note: coordinates.danger_level exists in the schema but is
// only ever written (randomized 1-5 on new-coordinate insert in
// internal/bot/handlers/onboarding.go and jobs.go) and never read by
// any game mechanic — the same "stored but mechanically dead" state
// world_state.active_weather was in before Phase 7 wired up
// world_events. This package does not build Snapshot fields around it;
// see ADR-018 in PROJECT_MASTER_PLAN.md.
package galaxyadvisor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// ContinentStatus is one continent's current environmental state.
type ContinentStatus struct {
	Continent string
	EventType string // "nominal" if clear — matches world.ActiveEventFor's own convention
}

// Snapshot is everything about the current galaxy state fed to the
// model, plus which continent the calling player calls home.
type Snapshot struct {
	HomeContinent string
	Continents    []ContinentStatus // ordered to match world.Continents
	RecentNews    []string          // most-recent-first, from world_news
}

// Action is one recommended next step.
type Action struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// Recommendation is the Galaxy Advisor's structured output.
type Recommendation struct {
	Summary             string   `json:"summary"`
	HomeContinentAdvice string   `json:"home_continent_advice"`
	GalaxyOutlook       string   `json:"galaxy_outlook"`
	RecommendedActions  []Action `json:"recommended_actions"`
	Notes               string   `json:"notes"`

	// FellBackToRawText is true when JSON parsing failed and Summary
	// holds the model's raw, unparsed text instead.
	FellBackToRawText bool
	// Truncated is true when the fallback happened because the
	// model's response was cut off mid-object (almost always meaning
	// MaxTokens was hit before the model finished). See ADR-016 in
	// PROJECT_MASTER_PLAN.md.
	Truncated bool
}

// eventEffect gives the mechanical-effect description for a
// world_events event_type, matching internal/engine/world/weather.go's
// own eventHeadline text. Deliberately duplicated rather than shared —
// weather.go's eventHeadline is unexported, and this codebase's
// existing pattern (see that function's own doc comment pointing at
// internal/bot/handlers/world.go's separate in-panel text) is to keep
// each consumer's copy of "what this event means" independently
// readable rather than adding a cross-package dependency for what's
// fundamentally presentation text. If a new event type is added to
// weather.go's eventPool, add it here too.
func eventEffect(eventType string) string {
	switch eventType {
	case "solar_flare":
		return "Solar generators running at 200% output; automation agents on standby."
	case "radiation_storm":
		return "Morale decay rate doubled."
	case "acid_rain":
		return "Active construction projects running at reduced speed."
	case "emp":
		return "Automation agents down; electricity generation offline."
	case "supply_crisis":
		return "Market Exchange sale prices depressed."
	case "disease":
		return "Rations consumption elevated."
	case "sandstorm":
		return "Scan and Scout operations report degraded intel accuracy."
	case "nominal", "":
		return "Conditions nominal — no active environmental effects."
	default:
		return "Unrecognized event type — no known mechanical effect on file."
	}
}

// SystemPrompt is the fixed instruction given to the model for every
// Galaxy Advisor call.
const SystemPrompt = `You are the AI Dynamic Galaxy advisor for a player in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: help the player understand the current environmental state of the galaxy — the world event (if any) active on their own continent, and the wider picture across all four continents (Africa, Europe, Asia, Americas) — and what that implies for near-term decisions. You NEVER take any action yourself — you only advise.

Rules:
- Ground every piece of advice in the specific event type and its stated mechanical effect — never generic "stay alert" advice untethered from what's actually happening.
- home_continent_advice must address the player's own continent specifically, even if it's currently nominal (say so plainly rather than skipping it).
- galaxy_outlook should look across all four continents together — e.g. call out if another continent is currently clear and might be a better target for a cross-continent raid, or if multiple continents share a similar event.
- World events expire on their own over time; don't claim to know exactly when one will end unless told so, and don't fabricate an end time.
- recommended_actions should be concrete next steps (e.g. "hold off on new construction until Acid Rain clears"), not vague platitudes. If nothing on the board warrants action right now, say so honestly rather than inventing one.
- Keep summary to 2-3 sentences.`

// BuildUserPrompt renders a Snapshot into the data block the model
// reasons over.
func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PLAYER'S HOME CONTINENT: %s\n\n", s.HomeContinent)

	b.WriteString("CONTINENT STATUS:\n")
	for _, cs := range s.Continents {
		home := ""
		if cs.Continent == s.HomeContinent {
			home = " (home)"
		}
		fmt.Fprintf(&b, "  %s%s: %s — %s\n", cs.Continent, home, cs.EventType, eventEffect(cs.EventType))
	}

	b.WriteString("\nRECENT SECTOR NEWS (most recent first):\n")
	if len(s.RecentNews) == 0 {
		b.WriteString("  No recent bulletins.\n")
	} else {
		for _, headline := range s.RecentNews {
			fmt.Fprintf(&b, "  - %s\n", headline)
		}
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "home_continent_advice": "...", "galaxy_outlook": "...", "recommended_actions": [{"action": "...", "reason": "..."}], "notes": "..."}`)

	return b.String()
}

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the same way every other Phase B-G package does.
func ParseRecommendation(text string) *Recommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
	}

	var rec Recommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	// See ADR-015: real providers occasionally leave a raw,
	// unescaped newline/tab inside a string value.
	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
}

// FormatForTelegram renders a Recommendation as a plain-text message
// suitable for a Telegram reply.
func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🌌 AI DYNAMIC GALAXY ADVISOR\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ The AI's response got cut off before it finished — showing the partial reply below:\n\n")
		} else {
			b.WriteString("⚠️ Couldn't parse the AI's structured response — showing its raw reply below:\n\n")
		}
		fmt.Fprintf(&b, "```\n%s\n```", rec.Summary)
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", rec.Summary)

	if rec.HomeContinentAdvice != "" {
		fmt.Fprintf(&b, "🏠 Home continent: %s\n\n", rec.HomeContinentAdvice)
	}
	if rec.GalaxyOutlook != "" {
		fmt.Fprintf(&b, "🌍 Galaxy outlook: %s\n\n", rec.GalaxyOutlook)
	}

	if len(rec.RecommendedActions) > 0 {
		b.WriteString("RECOMMENDED ACTIONS:\n")
		for i, a := range rec.RecommendedActions {
			fmt.Fprintf(&b, "%d. %s\n   💡 %s\n", i+1, a.Action, a.Reason)
		}
		b.WriteString("\n")
	}

	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", rec.Notes)
	}

	b.WriteString("\nThis is a briefing only — nothing has been moved, built, or changed automatically.")
	return b.String()
}
