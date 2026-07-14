// Package governor implements Phase B of the AI Systems Roadmap: the
// AI Planet Governor. Unlike internal/ai (which must stay game-agnostic
// per ADR-001 in PROJECT_MASTER_PLAN.md), this package is allowed to
// know about encampments, modules, and resources — it is the seam
// where game state meets the AI Foundation.
//
// This file intentionally contains zero I/O (no *sql.DB, no network)
// so it can be unit tested directly. All database access lives in
// governor.go.
package governor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ModuleState is a minimal projection of a row in the `modules` table.
type ModuleState struct {
	Type  string
	Level int
}

// Snapshot is everything about one player's base that the Governor
// feeds to the model. It deliberately mirrors existing DB concepts
// (see internal/engine/resource.EncampmentState) rather than inventing
// a parallel data model.
type Snapshot struct {
	EncampmentID string
	Name         string
	Level        int

	Scrap       float64
	Rations     float64
	Electricity float64
	Metal       float64
	Crystal     float64
	Hydrogen    float64
	Dollars     float64

	Modules []ModuleState

	Soldiers int
	Buggies  int
	Ships    int

	DefenseTechLvl    int
	ProductionTechLvl int
}

// Action is one recommended step, in the shape the model is asked to
// return inside Recommendation.PriorityActions.
type Action struct {
	Action string `json:"action"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

// Recommendation is the Governor's structured output. If the model's
// response can't be parsed as JSON (see ADR-005 in
// PROJECT_MASTER_PLAN.md — Anthropic has no enforced JSON mode),
// ParseRecommendation falls back to Summary=<raw text>, so the player
// always sees *something* useful rather than an error.
type Recommendation struct {
	Summary         string   `json:"summary"`
	PriorityActions []Action `json:"priority_actions"`
	StorageWarning  string   `json:"storage_warning"`
	ExpectedImpact  string   `json:"expected_impact"`
	// FellBackToRawText is true when JSON parsing failed and Summary
	// holds the model's raw, unparsed text instead.
	FellBackToRawText bool
}

// SystemPrompt is the fixed instruction given to the model for every
// Governor call. It stays outside the per-call BuildUserPrompt so it's
// easy to find and tune independent of the encampment data.
const SystemPrompt = `You are the AI Planet Governor for a player's base in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: analyze the player's current base state and recommend concrete next actions. You NEVER take action yourself — you only recommend. The player (or an explicit autopilot toggle they control elsewhere) decides whether to act.

Focus areas, in priority order:
1. Prevent resource storage overflow (wasted production).
2. Identify the single highest-value building/module to upgrade next, with reasoning.
3. Flag any building or unit that appears damaged, undersized, or bottlenecking growth.
4. Suggest a sensible construction/research queue order for the next few actions.
5. Note any production optimization (idle capacity, imbalanced resource mix).

Be specific and concrete — reference actual module names and levels from the data given, not generic advice. Keep the summary to 2-3 sentences. List at most 4 priority actions, ordered most-to-least important.`

// BuildUserPrompt renders a Snapshot into the human-readable data
// block the model reasons over. Kept deterministic (sorted module
// list) so identical snapshots produce identical prompts, which
// matters for the AI Foundation's cache layer (internal/ai.CacheKey
// hashes the full message list).
func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "BASE: %q (Level %d, ID %s)\n\n", s.Name, s.Level, s.EncampmentID)

	b.WriteString("RESOURCES:\n")
	fmt.Fprintf(&b, "  Scrap: %.1f | Rations: %.1f | Electricity: %.1f\n", s.Scrap, s.Rations, s.Electricity)
	fmt.Fprintf(&b, "  Metal: %.1f | Crystal: %.1f | Hydrogen: %.1f | Dollars: %.1f\n\n", s.Metal, s.Crystal, s.Hydrogen, s.Dollars)

	modules := append([]ModuleState(nil), s.Modules...)
	sort.Slice(modules, func(i, j int) bool { return modules[i].Type < modules[j].Type })
	b.WriteString("MODULES:\n")
	if len(modules) == 0 {
		b.WriteString("  (none built yet)\n")
	}
	for _, m := range modules {
		fmt.Fprintf(&b, "  %s: level %d\n", m.Type, m.Level)
	}

	fmt.Fprintf(&b, "\nMILITARY: %d soldiers, %d buggies, %d ships\n", s.Soldiers, s.Buggies, s.Ships)
	fmt.Fprintf(&b, "RESEARCH: defense tech lvl %d, production tech lvl %d\n", s.DefenseTechLvl, s.ProductionTechLvl)

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "priority_actions": [{"action": "...", "target": "...", "reason": "..."}], "storage_warning": "...", "expected_impact": "..."}`)

	return b.String()
}

// ParseRecommendation decodes the model's response text. It tolerates
// responses wrapped in markdown code fences (some models add them
// despite instructions not to) by stripping a leading/trailing ```
// fence before attempting to parse.
func ParseRecommendation(text string) *Recommendation {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var rec Recommendation
	if err := json.Unmarshal([]byte(cleaned), &rec); err != nil || rec.Summary == "" {
		return &Recommendation{Summary: text, FellBackToRawText: true}
	}
	return &rec
}

// FormatForTelegram renders a Recommendation as a plain-text message
// suitable for a Telegram reply.
func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🧠 AI PLANET GOVERNOR\n\n")

	if rec.FellBackToRawText {
		b.WriteString(rec.Summary)
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", rec.Summary)
	if len(rec.PriorityActions) > 0 {
		b.WriteString("PRIORITY ACTIONS:\n")
		for i, a := range rec.PriorityActions {
			fmt.Fprintf(&b, "%d. %s — %s\n   → %s\n", i+1, a.Action, a.Target, a.Reason)
		}
		b.WriteString("\n")
	}
	if rec.StorageWarning != "" {
		fmt.Fprintf(&b, "⚠️ Storage: %s\n", rec.StorageWarning)
	}
	if rec.ExpectedImpact != "" {
		fmt.Fprintf(&b, "📈 Expected impact: %s\n", rec.ExpectedImpact)
	}
	b.WriteString("\nThis is a recommendation only — nothing has been built, upgraded, or queued automatically.")
	return b.String()
}
