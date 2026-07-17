// Package researchplanner implements Phase E of the AI Systems Roadmap:
// the AI Research Planner. Like governor, fleetcommander, and
// econadvisor, this package is the seam where game state meets
// ai.Service; internal/ai itself stays game-agnostic per ADR-001 in
// PROJECT_MASTER_PLAN.md.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in planner.go.
//
// Note: this package deliberately does NOT import
// internal/bot/handlers (which owns the live /research panel and its
// upgrade-spend logic) or any sibling AI package. The 7-node tech
// tree's shape (key, db column, cost formula, max level) is a small,
// deliberately duplicated copy of internal/bot/handlers/research.go's
// researchTree — same trade-off already made for the mock provider's
// placeholder JSON (see internal/ai/providers/mock/provider.go): each
// AI feature package stays independently buildable/mergeable rather
// than forming a dependency chain. If the real tech tree ever changes
// (a node added/removed, cost formula changed), update both copies —
// this package's own tests (TestResearchNodes_MatchLiveHandler-style
// assertions in prompt_test.go) exist to catch drift.
package researchplanner

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// MaxResearchLevel mirrors handlers.MaxResearchLevel. Kept as an
// independent constant per the package doc comment above.
const MaxResearchLevel = 20

// Goal is a player-selectable (or inferred) strategic focus that
// biases which tech nodes the planner prioritizes.
type Goal string

const (
	GoalRaiding  Goal = "raiding"
	GoalDefense  Goal = "defense"
	GoalEconomy  Goal = "economy"
	GoalBalanced Goal = "balanced"
)

// ValidGoals lists every goal accepted from a callback/command
// argument. Anything else falls back to inference.
func ValidGoals() []Goal {
	return []Goal{GoalRaiding, GoalDefense, GoalEconomy, GoalBalanced}
}

// IsValidGoal reports whether s names a known Goal.
func IsValidGoal(s string) bool {
	for _, g := range ValidGoals() {
		if string(g) == s {
			return true
		}
	}
	return false
}

// TechNode is a minimal projection of one branch of the real 7-node
// tech tree (see handlers.researchTree). Order here matches the live
// panel's display order.
type TechNode struct {
	Key         string // matches handlers.researchNode.key / dbColumn suffix
	Title       string
	Description string
	Level       int
	Cost        int // Neuro Cores to advance from Level to Level+1; 0 if maxed
	Maxed       bool
}

// techMeta is the static (key, title, description) triple for each of
// the 7 real tech nodes, in the live panel's display order.
var techMeta = []struct {
	key, title, desc string
}{
	{"econ", "Technology", "Reduces Automated Agent electricity consumption."},
	{"production", "Production", "Increases passive Scrap Heap mining speed."},
	{"integrity", "Integrity", "Reduces casualties suffered by your units in combat."},
	{"defense", "Shields", "Strengthens your Outpost's defensive rating against raids."},
	{"intel", "Intelligence", "Improves spy satellite intercept odds & counter-intel."},
	{"speed", "Thrusters", "Reduces march/travel time for raids and scouts."},
	{"military", "Weapons", "Multiplies Mech and offensive unit combat ratings."},
}

// TechNodeCost returns the Neuro Core cost to advance from currentLvl
// to currentLvl+1 — the same linear formula (level × 8) as
// handlers.researchCost.
func TechNodeCost(currentLvl int) int {
	return currentLvl * 8
}

// BuildTechNodes turns a raw key→level map (as returned by the live
// research_states row) into the ordered []TechNode the prompt and
// Telegram renderer both use.
func BuildTechNodes(levels map[string]int) []TechNode {
	nodes := make([]TechNode, 0, len(techMeta))
	for _, m := range techMeta {
		lvl := levels[m.key]
		n := TechNode{Key: m.key, Title: m.title, Description: m.desc, Level: lvl}
		if lvl >= MaxResearchLevel {
			n.Maxed = true
		} else {
			n.Cost = TechNodeCost(lvl)
		}
		nodes = append(nodes, n)
	}
	return nodes
}

// Snapshot is everything about one player's tech tree fed to the
// model.
type Snapshot struct {
	EncampmentID string
	Name         string
	Level        int

	NeuroCores float64
	Nodes      []TechNode

	// RequestedGoal is what the player explicitly picked, if
	// anything; empty means "infer it."
	RequestedGoal Goal
}

// InferredGoal returns RequestedGoal if the player set one, otherwise
// infers a reasonable default from the current tech spread: whichever
// non-maxed node cluster is furthest behind the others gets
// prioritized as "balanced" catch-up, unless every combat-adjacent
// node (integrity/defense/military) is already ahead of
// econ/production, in which case "economy" is inferred to avoid
// recommending more of what's already ahead.
func (s Snapshot) InferredGoal() Goal {
	if s.RequestedGoal != "" {
		return s.RequestedGoal
	}

	var combatTotal, economyTotal, combatCount, economyCount int
	for _, n := range s.Nodes {
		switch n.Key {
		case "integrity", "defense", "military":
			combatTotal += n.Level
			combatCount++
		case "econ", "production":
			economyTotal += n.Level
			economyCount++
		}
	}
	if combatCount == 0 || economyCount == 0 {
		return GoalBalanced
	}
	combatAvg := float64(combatTotal) / float64(combatCount)
	economyAvg := float64(economyTotal) / float64(economyCount)

	// A meaningful (not noise-level) lead one way or the other
	// nudges the inferred goal toward evening things out.
	const driftThreshold = 2.0
	switch {
	case combatAvg-economyAvg >= driftThreshold:
		return GoalEconomy
	case economyAvg-combatAvg >= driftThreshold:
		return GoalDefense
	default:
		return GoalBalanced
	}
}

// PrioritizedNode is one step in the recommended research order.
type PrioritizedNode struct {
	Node         string `json:"node"`
	TargetLevel  int    `json:"target_level"`
	Reason       string `json:"reason"`
	CoreCost     int    `json:"core_cost"`
	ExpectedGain string `json:"expected_gain"`
}

// Recommendation is the Research Planner's structured output.
type Recommendation struct {
	Summary          string            `json:"summary"`
	GoalUsed         string            `json:"goal_used"`
	RecommendedOrder []PrioritizedNode `json:"recommended_order"`
	CoresNeeded      int               `json:"cores_needed"`
	CoresAvailable   int               `json:"cores_available"`
	Notes            string            `json:"notes"`

	// FellBackToRawText is true when JSON parsing failed and Summary
	// holds the model's raw, unparsed text instead.
	FellBackToRawText bool
}

const SystemPrompt = `You are the AI Research Planner for a player's base in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: given the player's current tech tree levels, Neuro Core stockpile, and a stated (or inferred) strategic goal, recommend the best order to spend Neuro Cores across the 7 research nodes. You NEVER research anything yourself — you only recommend.

Rules:
- The 7 nodes are: econ (Technology, reduces Automated Agent electricity use), production (passive mining speed), integrity (reduces combat casualties), defense (Outpost defensive rating), intel (spy/counter-intel), speed (march/travel time), military (offensive combat rating).
- Research has NO queue and NO timer — each level-up is an instant spend, so the only real tradeoff is which order to spend limited Neuro Cores in, not scheduling.
- Tailor the recommended order to the stated goal:
  - raiding: prioritize speed and military, with intel close behind.
  - defense: prioritize defense and integrity, with intel close behind.
  - economy: prioritize econ and production.
  - balanced: spread recommendations roughly evenly, favoring whichever nodes are furthest behind the others.
- Never recommend a node already at max level.
- List at most 5 steps in recommended_order, ordered highest-priority first, each with a concrete core_cost (must match the provided per-node cost for that node's current level) and a brief expected_gain (e.g. "raid travel time -8%").
- cores_needed is the sum of core_cost across every step you list; cores_available is the player's current Neuro Core count, taken as given (do not recompute it).
- If cores_available is less than cores_needed, say so plainly in notes rather than pretending the player can afford everything at once.
- Keep summary to 2-3 sentences.`

// BuildUserPrompt renders a Snapshot into the data block the model
// reasons over. Sorted/stable iteration keeps output deterministic for
// internal/ai's cache key hashing.
func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "BASE: %q (Level %d, ID %s)\n\n", s.Name, s.Level, s.EncampmentID)
	fmt.Fprintf(&b, "GOAL: %s\n\n", s.InferredGoal())
	fmt.Fprintf(&b, "NEURO CORES AVAILABLE: %.0f\n\n", s.NeuroCores)

	b.WriteString("TECH TREE (7 nodes, max level 20, no queue/timer, cost = current_level × 8 cores):\n")
	nodes := append([]TechNode(nil), s.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Key < nodes[j].Key })
	for _, n := range nodes {
		if n.Maxed {
			fmt.Fprintf(&b, "  %s (%s): level %d/%d — MAX, cannot be upgraded further\n", n.Key, n.Title, n.Level, MaxResearchLevel)
			continue
		}
		fmt.Fprintf(&b, "  %s (%s): level %d/%d — next level costs %d cores. %s\n",
			n.Key, n.Title, n.Level, MaxResearchLevel, n.Cost, n.Description)
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "goal_used": "...", "recommended_order": [{"node": "...", "target_level": 0, "reason": "...", "core_cost": 0, "expected_gain": "..."}], "cores_needed": 0, "cores_available": 0, "notes": "..."}`)

	return b.String()
}

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the same way governor/fleetcommander/econadvisor
// do.
func ParseRecommendation(text string) *Recommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &Recommendation{Summary: text, FellBackToRawText: true}
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

	return &Recommendation{Summary: text, FellBackToRawText: true}
}

// FormatForTelegram renders a Recommendation as a plain-text message.
func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🧪 AI RESEARCH PLANNER\n\n")

	if rec.FellBackToRawText {
		b.WriteString("⚠️ Couldn't parse the AI's structured response — showing its raw reply below:\n\n")
		fmt.Fprintf(&b, "```\n%s\n```", rec.Summary)
		return b.String()
	}

	if rec.GoalUsed != "" {
		fmt.Fprintf(&b, "🎯 Goal: %s\n\n", rec.GoalUsed)
	}
	fmt.Fprintf(&b, "📋 %s\n\n", rec.Summary)

	if len(rec.RecommendedOrder) > 0 {
		b.WriteString("RECOMMENDED RESEARCH ORDER:\n")
		for i, n := range rec.RecommendedOrder {
			fmt.Fprintf(&b, "%d. %s → Lvl %d (%d cores)\n   → %s (%s)\n",
				i+1, n.Node, n.TargetLevel, n.CoreCost, n.Reason, n.ExpectedGain)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "🧠 Cores needed: %d | Available: %d\n", rec.CoresNeeded, rec.CoresAvailable)
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", rec.Notes)
	}

	b.WriteString("\nThis is a recommendation only — nothing has been researched automatically.")
	return b.String()
}
