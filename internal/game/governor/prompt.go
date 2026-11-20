package governor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type ModuleState struct {
	Type  string
	Level int
}

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

type Action struct {
	Action string `json:"action"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type Recommendation struct {
	Summary           string   `json:"summary"`
	PriorityActions   []Action `json:"priority_actions"`
	StorageWarning    string   `json:"storage_warning"`
	ExpectedImpact    string   `json:"expected_impact"`
	FellBackToRawText bool
	Truncated         bool
}

const SystemPrompt = `You are the AI Planet Governor for a player's base in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: analyze the player's current base state and recommend concrete next actions. You NEVER take action yourself — you only recommend. The player (or an explicit autopilot toggle they control elsewhere) decides whether to act.

Focus areas, in priority order:
1. Prevent resource storage overflow (wasted production).
2. Identify the single highest-value building/module to upgrade next, with reasoning.
3. Flag any building or unit that appears damaged, undersized, or bottlenecking growth.
4. Suggest a sensible construction/research queue order for the next few actions.
5. Note any production optimization (idle capacity, imbalanced resource mix).

Be specific and concrete — reference actual module names and levels from the data given, not generic advice. Keep the summary to 2-3 sentences. List at most 4 priority actions, ordered most-to-least important.`

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

func ParseRecommendation(text string, stopReason string) *Recommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
	}

	var rec Recommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
}

func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🧠 " + ai.HTMLBold("AI PLANET GOVERNOR") + "\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ " + ai.HTMLItalic("The AI's response got cut off before it finished — tap below to expand the partial reply:") + "\n\n")
		} else {
			b.WriteString("⚠️ " + ai.HTMLItalic("Couldn't parse the AI's structured response — tap below to expand its raw reply:") + "\n\n")
		}
		b.WriteString(ai.HTMLExpandableQuote(ai.HTMLEscape(rec.Summary)))
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", ai.HTMLEscape(rec.Summary))
	if len(rec.PriorityActions) > 0 {
		b.WriteString("🎯 " + ai.HTMLBold("PRIORITY ACTIONS") + "\n")
		for i, a := range rec.PriorityActions {
			fmt.Fprintf(&b, "%d. %s — %s\n   ➜ %s\n", i+1, ai.HTMLBold(ai.HTMLEscape(a.Action)), ai.HTMLEscape(a.Target), ai.HTMLEscape(a.Reason))
		}
		b.WriteString("\n")
	}
	if rec.StorageWarning != "" {
		fmt.Fprintf(&b, "⚠️ Storage: %s\n", ai.HTMLEscape(rec.StorageWarning))
	}
	if rec.ExpectedImpact != "" {
		fmt.Fprintf(&b, "📈 Expected impact: %s\n", ai.HTMLEscape(rec.ExpectedImpact))
	}
	b.WriteString("\n" + ai.HTMLItalic("This is a recommendation only — nothing has been built, upgraded, or queued automatically."))
	return b.String()
}
