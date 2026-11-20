package battleanalyst

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type RaidStats struct {
	TotalRaids       int
	ApparentWins     int
	TotalLosses      int
	AverageLosses    float64
	TotalStolenValue float64
}

type ArenaStats struct {
	Wins   int
	Losses int
}

type Snapshot struct {
	EncampmentID string
	Name         string
	Level        int

	AsAttacker RaidStats
	AsDefender RaidStats
	Arena      ArenaStats
}

type Pattern struct {
	Observation string `json:"observation"`
	Evidence    string `json:"evidence"`
	Suggestion  string `json:"suggestion"`
}

type Recommendation struct {
	Summary          string    `json:"summary"`
	KeyPatterns      []Pattern `json:"key_patterns"`
	RecommendedFocus string    `json:"recommended_focus"`
	Notes            string    `json:"notes"`

	FellBackToRawText bool
	Truncated         bool
}

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
	b.WriteString("📊 " + ai.HTMLBold("AI BATTLE ANALYST") + "\n\n")

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

	if len(rec.KeyPatterns) > 0 {
		b.WriteString("🔎 " + ai.HTMLBold("KEY PATTERNS") + "\n")
		for i, p := range rec.KeyPatterns {
			fmt.Fprintf(&b, "%d. %s\n   📎 %s\n   💡 %s\n", i+1, ai.HTMLBold(ai.HTMLEscape(p.Observation)), ai.HTMLEscape(p.Evidence), ai.HTMLEscape(p.Suggestion))
		}
		b.WriteString("\n")
	}

	if rec.RecommendedFocus != "" {
		fmt.Fprintf(&b, "🎯 Recommended focus: %s\n", ai.HTMLEscape(rec.RecommendedFocus))
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", ai.HTMLEscape(rec.Notes))
	}

	b.WriteString("\n" + ai.HTMLItalic("This is an analysis of past battles only — nothing has been changed automatically."))
	return b.String()
}
