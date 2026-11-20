package researchplanner

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

const MaxResearchLevel = 20

type Goal string

const (
	GoalRaiding  Goal = "raiding"
	GoalDefense  Goal = "defense"
	GoalEconomy  Goal = "economy"
	GoalBalanced Goal = "balanced"
)

func ValidGoals() []Goal {
	return []Goal{GoalRaiding, GoalDefense, GoalEconomy, GoalBalanced}
}

func IsValidGoal(s string) bool {
	for _, g := range ValidGoals() {
		if string(g) == s {
			return true
		}
	}
	return false
}

type TechNode struct {
	Key         string
	Title       string
	Description string
	Level       int
	Cost        int
	Maxed       bool
}

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

func TechNodeCost(currentLvl int) int {
	return currentLvl * 8
}

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

type Snapshot struct {
	EncampmentID string
	Name         string
	Level        int

	NeuroCores float64
	Nodes      []TechNode

	RequestedGoal Goal
}

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

type PrioritizedNode struct {
	Node         string `json:"node"`
	TargetLevel  int    `json:"target_level"`
	Reason       string `json:"reason"`
	CoreCost     int    `json:"core_cost"`
	ExpectedGain string `json:"expected_gain"`
}

type Recommendation struct {
	Summary          string            `json:"summary"`
	GoalUsed         string            `json:"goal_used"`
	RecommendedOrder []PrioritizedNode `json:"recommended_order"`
	CoresNeeded      int               `json:"cores_needed"`
	CoresAvailable   int               `json:"cores_available"`
	Notes            string            `json:"notes"`

	FellBackToRawText bool
	Truncated         bool
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
	b.WriteString("🧪 " + ai.HTMLBold("AI RESEARCH PLANNER") + "\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ " + ai.HTMLItalic("The AI's response got cut off before it finished — tap below to expand the partial reply:") + "\n\n")
		} else {
			b.WriteString("⚠️ " + ai.HTMLItalic("Couldn't parse the AI's structured response — tap below to expand its raw reply:") + "\n\n")
		}
		b.WriteString(ai.HTMLExpandableQuote(ai.HTMLEscape(rec.Summary)))
		return b.String()
	}

	if rec.GoalUsed != "" {
		fmt.Fprintf(&b, "🎯 Goal: %s\n\n", ai.HTMLEscape(rec.GoalUsed))
	}
	fmt.Fprintf(&b, "📋 %s\n\n", ai.HTMLEscape(rec.Summary))

	if len(rec.RecommendedOrder) > 0 {
		b.WriteString("🔬 " + ai.HTMLBold("RECOMMENDED RESEARCH ORDER") + "\n")
		for i, n := range rec.RecommendedOrder {
			fmt.Fprintf(&b, "%d. %s ➜ %s\n   %s (%s)\n",
				i+1, ai.HTMLBold(ai.HTMLEscape(n.Node)), ai.HTMLCode(fmt.Sprintf("Lvl %d — %d cores", n.TargetLevel, n.CoreCost)), ai.HTMLEscape(n.Reason), ai.HTMLEscape(n.ExpectedGain))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "🧠 Cores needed: %s | Available: %s\n", ai.HTMLCode(fmt.Sprintf("%d", rec.CoresNeeded)), ai.HTMLCode(fmt.Sprintf("%d", rec.CoresAvailable)))
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", ai.HTMLEscape(rec.Notes))
	}

	b.WriteString("\n" + ai.HTMLItalic("This is a recommendation only — nothing has been researched automatically."))
	return b.String()
}
