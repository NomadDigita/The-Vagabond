// Package battleanalyst implements Phase F of the AI Systems Roadmap:
// the AI Battle Analyst. Like governor, fleetcommander, econadvisor,
// and researchplanner, this package is the seam where game state meets
// ai.Service; internal/ai itself stays game-agnostic per ADR-001 in
// PROJECT_MASTER_PLAN.md.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in analyst.go.
//
// Unlike Fleet Commander (Phase C), which is forward-looking — "should
// I attack this specific target right now?" — Battle Analyst is
// backward-looking: it reviews a player's accumulated combat record
// across raids (as both attacker and defender) and arena battles, and
// surfaces recurring patterns the player might not notice from
// scrolling through individual battle reports one at a time.
//
// Scope note: this package deliberately does NOT analyze World Boss
// engagements. See ADR-017 in PROJECT_MASTER_PLAN.md — a schema audit
// done before writing this file (not assumed from an older session's
// notes) confirmed world_boss_attacks rows are deleted once survivors
// return home, and world_boss_contributions rows are deleted the
// moment a boss is defeated. Neither table retains any history across
// completed engagements, so there is nothing durable for this package
// to read for World Bosses today.
package battleanalyst

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// RaidStats summarizes a player's completed raids from one side of the
// fight (attacker or defender). "Apparent" outcomes are the same
// stolen-resources heuristic fleetcommander.BuildCombatHistory already
// uses (raids has no explicit outcome column) — see that package's
// doc comment for why this is flagged as approximate.
type RaidStats struct {
	TotalRaids       int
	ApparentWins     int
	TotalLosses      int
	AverageLosses    float64
	TotalStolenValue float64 // attacker side only; always 0 for defender stats
}

// ArenaStats summarizes a player's arena_battles rows, matched by
// their current username (see analyst.go for the caveat this implies
// about username changes).
type ArenaStats struct {
	Wins   int
	Losses int
}

// Snapshot is everything about one player's combat record fed to the
// model.
type Snapshot struct {
	EncampmentID string
	Name         string
	Level        int

	AsAttacker RaidStats
	AsDefender RaidStats
	Arena      ArenaStats
}

// Pattern is one recurring observation the model identifies, in the
// shape it's asked to return inside Recommendation.KeyPatterns.
type Pattern struct {
	Observation string `json:"observation"`
	Evidence    string `json:"evidence"`
	Suggestion  string `json:"suggestion"`
}

// Recommendation is the Battle Analyst's structured output. If the
// model's response can't be parsed as JSON, ParseRecommendation falls
// back to Summary=<raw text>, matching every other Phase B-J package.
type Recommendation struct {
	Summary          string    `json:"summary"`
	KeyPatterns      []Pattern `json:"key_patterns"`
	RecommendedFocus string    `json:"recommended_focus"`
	Notes            string    `json:"notes"`

	// FellBackToRawText is true when JSON parsing failed and Summary
	// holds the model's raw, unparsed text instead.
	FellBackToRawText bool
	// Truncated is true when the fallback happened because the
	// model's response was cut off mid-object (almost always meaning
	// MaxTokens was hit before the model finished). See ADR-016 in
	// PROJECT_MASTER_PLAN.md.
	Truncated bool
}

// SystemPrompt is the fixed instruction given to the model for every
// Battle Analyst call.
const SystemPrompt = `You are the AI Battle Analyst for a player in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: review the player's accumulated combat record — raids they've launched as attacker, raids launched against them as defender, and arena battles — and identify recurring patterns the player might not notice from reading individual battle reports one at a time. You NEVER launch, retreat, or change anything yourself — you only analyze what already happened.

Rules:
- Ground every observation in the actual numbers given — never generic combat advice untethered from this player's specific record.
- If a sample size is small (e.g. fewer than 5 raids on a side), say so explicitly rather than drawing a confident conclusion from too little data.
- Distinguish attacker-side and defender-side patterns clearly — a player can be a strong attacker and a weak defender (or vice versa), and conflating the two is misleading.
- If arena and raid results tell different stories (e.g. strong 1v1 arena record but weak raid defense), call that contrast out explicitly — it usually points at a specific gap (garrison strength vs. hero/unit quality).
- List at most 4 key patterns, ordered most-to-least actionable.
- recommended_focus should be one concrete, specific next step (e.g. "reinforce defensive garrison before your next offensive push"), not a vague platitude.
- If a side (attacker/defender/arena) has zero recorded battles, say so plainly in notes rather than fabricating a pattern for it.
- Keep summary to 2-3 sentences.`

// BuildUserPrompt renders a Snapshot into the data block the model
// reasons over.
func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PLAYER BASE: %q (Level %d, ID %s)\n\n", s.Name, s.Level, s.EncampmentID)

	b.WriteString("RAIDS AS ATTACKER (completed only):\n")
	writeRaidStats(&b, s.AsAttacker, true)

	b.WriteString("\nRAIDS AS DEFENDER (completed only):\n")
	writeRaidStats(&b, s.AsDefender, false)

	b.WriteString("\nARENA BATTLES:\n")
	if s.Arena.Wins+s.Arena.Losses == 0 {
		b.WriteString("  No recorded arena battles.\n")
	} else {
		fmt.Fprintf(&b, "  %d wins, %d losses\n", s.Arena.Wins, s.Arena.Losses)
	}

	b.WriteString("\nNote: World Boss engagement history is not available — the game's data model does not retain it across completed engagements.\n")

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "key_patterns": [{"observation": "...", "evidence": "...", "suggestion": "..."}], "recommended_focus": "...", "notes": "..."}`)

	return b.String()
}

func writeRaidStats(b *strings.Builder, stats RaidStats, isAttacker bool) {
	if stats.TotalRaids == 0 {
		b.WriteString("  No recorded raids.\n")
		return
	}
	fmt.Fprintf(b, "  %d raids, %d apparent wins, %.1f average losses per raid\n",
		stats.TotalRaids, stats.ApparentWins, stats.AverageLosses)
	if isAttacker {
		fmt.Fprintf(b, "  Total resources stolen across these raids: %.0f\n", stats.TotalStolenValue)
	}
}

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the same way every other Phase B-J package does.
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
	b.WriteString("📊 AI BATTLE ANALYST\n\n")

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

	if len(rec.KeyPatterns) > 0 {
		b.WriteString("KEY PATTERNS:\n")
		for i, p := range rec.KeyPatterns {
			fmt.Fprintf(&b, "%d. %s\n   📎 %s\n   💡 %s\n", i+1, p.Observation, p.Evidence, p.Suggestion)
		}
		b.WriteString("\n")
	}

	if rec.RecommendedFocus != "" {
		fmt.Fprintf(&b, "🎯 Recommended focus: %s\n", rec.RecommendedFocus)
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", rec.Notes)
	}

	b.WriteString("\nThis is an analysis of past battles only — nothing has been changed automatically.")
	return b.String()
}
