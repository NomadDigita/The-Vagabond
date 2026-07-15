// Package fleetcommander implements Phase C of the AI Systems Roadmap:
// the AI Fleet Commander. Like internal/game/governor, this package is
// the seam where game state meets the ai.Service — internal/ai itself
// stays game-agnostic per ADR-001 in PROJECT_MASTER_PLAN.md.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in commander.go.
package fleetcommander

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FleetComposition is a generic unit-name → count map, deliberately
// not a hardcoded struct field per unit type. workshop_inventory has
// grown unit columns over time (see cmd/bot/main.go's ALTER TABLE
// history) and will keep growing as the parallel SpaceHunt Phase 6
// (Combat) branch adds new ship/turret types — a map means this
// package never needs a code change just because a new unit column
// was added upstream.
type FleetComposition map[string]int

// TargetProfile describes what the fleet might be sent against.
type TargetProfile struct {
	Name        string
	IsPvE       bool
	ThreatTier  string // human-readable, e.g. "Moderate", "Severe" (PvE only)
	Garrison    FleetComposition
	TurretBonus float64 // flat defense-rating modifier bonus, 0 if none
}

// CombatHistorySummary is a rough win/loss proxy built from completed
// raids where the player was the attacker. See BuildCombatHistory in
// commander.go for the exact (heuristic) definition of "win" — the
// `raids` table has no explicit outcome column, so this is an
// approximation flagged as technical debt in PROJECT_MASTER_PLAN.md.
type CombatHistorySummary struct {
	RaidsAnalyzed int
	ApparentWins  int
	TotalLosses   int // summed attacker_losses across analyzed raids
	AverageLosses float64
}

// Action mirrors the roadmap's required recommendation vocabulary:
// attack, retreat, reinforce, scout, wait, split fleets.
type Action string

const (
	ActionAttack    Action = "attack"
	ActionRetreat   Action = "retreat"
	ActionReinforce Action = "reinforce"
	ActionScout     Action = "scout"
	ActionWait      Action = "wait"
	ActionSplit     Action = "split_fleet"
)

// Recommendation is the Fleet Commander's structured output.
type Recommendation struct {
	Recommendation string `json:"recommendation"` // one of the Action constants, as free text from the model
	Confidence     string `json:"confidence"`     // e.g. "high", "medium", "low"
	Reasoning      string `json:"reasoning"`
	RiskAssessment string `json:"risk_assessment"`
	SuggestedSplit string `json:"suggested_split"` // only meaningful if Recommendation == split_fleet

	// FellBackToRawText is true when JSON parsing failed and
	// Reasoning holds the model's raw, unparsed text instead.
	FellBackToRawText bool
}

const SystemPrompt = `You are the AI Fleet Commander for a player in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: analyze the player's fleet composition against a specific target's known or estimated garrison, plus the player's recent combat history, and recommend exactly one of these actions: attack, retreat, reinforce, scout, wait, split_fleet.

You NEVER launch anything yourself — you only recommend. The player always decides.

Rules:
- Always explain WHY, referencing actual unit counts and the target's known garrison — never generic advice.
- If the player's fleet is clearly outmatched, recommend retreat, reinforce, or scout — never attack just because the player seems to want to.
- If recent combat history shows a losing streak, factor that into your risk assessment explicitly.
- If you recommend split_fleet, describe roughly what to send vs. hold back in suggested_split.
- Be honest about uncertainty — if the target's garrison is only a rough estimate (as with a scanned rogue nest), say so in risk_assessment.`

// BuildUserPrompt renders own fleet + target + history into the data
// block the model reasons over. Sorted map iteration keeps output
// deterministic for internal/ai's cache key hashing.
func BuildUserPrompt(own FleetComposition, target TargetProfile, history CombatHistorySummary) string {
	var b strings.Builder

	b.WriteString("YOUR FLEET:\n")
	writeComposition(&b, own)

	b.WriteString("\nTARGET: ")
	if target.IsPvE {
		fmt.Fprintf(&b, "%s (PvE, estimated garrison, threat tier: %s)\n", target.Name, target.ThreatTier)
	} else {
		fmt.Fprintf(&b, "%s (rival player base)\n", target.Name)
	}
	writeComposition(&b, target.Garrison)
	if target.TurretBonus > 0 {
		fmt.Fprintf(&b, "  Defensive bonus: +%.0f%% (dug-in position)\n", target.TurretBonus*100)
	}

	b.WriteString("\nRECENT COMBAT HISTORY:\n")
	if history.RaidsAnalyzed == 0 {
		b.WriteString("  No recorded raids yet.\n")
	} else {
		fmt.Fprintf(&b, "  %d raids analyzed, %d apparent wins, %.1f average losses per raid\n",
			history.RaidsAnalyzed, history.ApparentWins, history.AverageLosses)
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"recommendation": "attack|retreat|reinforce|scout|wait|split_fleet", "confidence": "high|medium|low", "reasoning": "...", "risk_assessment": "...", "suggested_split": "..."}`)

	return b.String()
}

func writeComposition(b *strings.Builder, comp FleetComposition) {
	if len(comp) == 0 {
		b.WriteString("  (no units)\n")
		return
	}
	names := make([]string, 0, len(comp))
	for name := range comp {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		count := comp[name]
		if count <= 0 {
			continue
		}
		fmt.Fprintf(b, "  %s: %d\n", name, count)
	}
}

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the way governor.ParseRecommendation does. On
// failure it falls back to Reasoning=<raw text> so the player always
// gets something usable.
func ParseRecommendation(text string) *Recommendation {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var rec Recommendation
	if err := json.Unmarshal([]byte(cleaned), &rec); err != nil || rec.Recommendation == "" {
		return &Recommendation{Reasoning: text, FellBackToRawText: true}
	}
	return &rec
}

// actionEmoji gives each recommendation a distinct glyph so players
// can scan the outcome at a glance before reading the reasoning.
var actionEmoji = map[string]string{
	string(ActionAttack):    "⚔️",
	string(ActionRetreat):   "🏳️",
	string(ActionReinforce): "🛡️",
	string(ActionScout):     "🔭",
	string(ActionWait):      "⏳",
	string(ActionSplit):     "🔀",
}

// FormatForTelegram renders a Recommendation as a plain-text message.
func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🎖️ AI FLEET COMMANDER\n\n")

	if rec.FellBackToRawText {
		b.WriteString(rec.Reasoning)
		return b.String()
	}

	emoji := actionEmoji[strings.ToLower(rec.Recommendation)]
	if emoji == "" {
		emoji = "🎯"
	}
	fmt.Fprintf(&b, "%s RECOMMENDATION: %s (confidence: %s)\n\n", emoji, strings.ToUpper(rec.Recommendation), rec.Confidence)
	fmt.Fprintf(&b, "💭 Reasoning: %s\n", rec.Reasoning)
	if rec.RiskAssessment != "" {
		fmt.Fprintf(&b, "⚠️ Risk: %s\n", rec.RiskAssessment)
	}
	if strings.EqualFold(rec.Recommendation, string(ActionSplit)) && rec.SuggestedSplit != "" {
		fmt.Fprintf(&b, "🔀 Suggested split: %s\n", rec.SuggestedSplit)
	}
	b.WriteString("\nThis is a recommendation only — no fleet has been launched automatically.")
	return b.String()
}
