// Package guildassistant implements Phase G of the AI Systems Roadmap:
// the AI Guild Assistant. Like every other Phase B-F package, this is
// the seam where game state meets ai.Service; internal/ai itself stays
// game-agnostic per ADR-001.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in assistant.go.
//
// Scope, decided by reading the real "clans" system (the SpaceHunt
// Phase 2 branch's guild feature, see migrations/017) before writing
// any Snapshot fields — the same discipline Phase F used for its own
// schema audit:
//   - Clan roster health: member count vs. the 15-member cap, combined
//     level, combined military power (soldiers/mechs — the same
//     formula HandleAllianceStatsCallback already uses), and a count
//     of members who haven't been active recently.
//   - Pending recruitment applications, each with the applicant's
//     current outpost level, so the model can reason about fit rather
//     than just a name.
//   - Clan war record: durable history (clan_wars rows are never
//     deleted — confirmed by grepping for DELETE FROM clan_wars,
//     exactly as Phase F did for its own tables) plus the current
//     active war's live score if one is underway.
//
// This package is deliberately Leader-facing only (see assistant.go):
// every other clan-leadership action in internal/bot/handlers/clan.go
// (accepting/rejecting applicants, declaring war, kicking members) is
// already Leader-gated, and recruitment/war strategy is exactly that
// kind of decision — a regular member asking this would be reading
// information about applicants and strategy that isn't theirs to act
// on.
package guildassistant

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// Applicant is one pending clan_applications row, enriched with the
// applicant's current outpost level.
type Applicant struct {
	Username string
	Level    int
}

// WarRecord summarizes a clan's clan_wars history.
type WarRecord struct {
	CompletedWars int
	Wins          int
	Losses        int

	// InActiveWar, OpponentName, OurScore, TheirScore describe the
	// clan's current war if status='active' right now (at most one,
	// per HandleDeclareClanWarCallback's own "already at war" guard).
	InActiveWar  bool
	OpponentName string
	OurScore     float64
	TheirScore   float64
}

// Snapshot is everything about one clan fed to the model.
type Snapshot struct {
	ClanID     string
	Name       string
	Recruiting bool

	MemberCount     int
	MemberCap       int
	CombinedLevel   int
	MilitaryPower   int
	InactiveMembers int // no last_active within the lookback window used by assistant.go

	PendingApplicants []Applicant
	War               WarRecord
}

// RecruitmentCall is the model's per-applicant judgment.
type RecruitmentCall struct {
	Username       string `json:"username"`
	Recommendation string `json:"recommendation"` // "accept" | "reject" | "undecided"
	Reason         string `json:"reason"`
}

// Recommendation is the Guild Assistant's structured output.
type Recommendation struct {
	Summary          string            `json:"summary"`
	RecruitmentCalls []RecruitmentCall `json:"recruitment_calls"`
	WarInsight       string            `json:"war_insight"`
	RecommendedFocus string            `json:"recommended_focus"`
	Notes            string            `json:"notes"`

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
// Guild Assistant call.
const SystemPrompt = `You are the AI Guild Assistant for a Clan Leader in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: help the Leader make two kinds of decisions — which pending recruitment applicants to accept or reject, and how to think about their clan's current or recent war record. You NEVER accept, reject, or declare anything yourself — you only recommend.

Rules:
- Ground every recruitment call in the applicant's level relative to the clan's own average (CombinedLevel / MemberCount) — a much lower-level applicant isn't automatically a bad pick (they may just be new), but say so explicitly rather than treating all applicants the same.
- If the clan is at or near its 15-member cap, say so and weigh that against accepting more members.
- If MemberCount is 0 or PendingApplicants is empty, do not fabricate a recruitment_calls entry — return an empty list and say so in notes.
- For war_insight: if InActiveWar, comment on the live score gap and what it implies; if not, comment on the completed win/loss record instead (or say there isn't one yet if CompletedWars is 0). Don't invent urgency where the numbers don't show any.
- If InactiveMembers is a meaningful fraction of MemberCount, mention it as a roster-health concern — inactive members can't contribute to war score.
- recommended_focus should be one concrete next step for the Leader, not a vague platitude.
- Keep summary to 2-3 sentences.`

// BuildUserPrompt renders a Snapshot into the data block the model
// reasons over.
func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CLAN: %q (Recruiting: %t)\n", s.Name, s.Recruiting)
	fmt.Fprintf(&b, "Roster: %d / %d members, combined level %d, military power %d, %d inactive member(s)\n\n",
		s.MemberCount, s.MemberCap, s.CombinedLevel, s.MilitaryPower, s.InactiveMembers)

	b.WriteString("PENDING APPLICANTS:\n")
	if len(s.PendingApplicants) == 0 {
		b.WriteString("  None.\n")
	} else {
		for _, a := range s.PendingApplicants {
			fmt.Fprintf(&b, "  @%s — Level %d\n", a.Username, a.Level)
		}
	}

	b.WriteString("\nWAR RECORD:\n")
	if s.War.InActiveWar {
		fmt.Fprintf(&b, "  Currently at war with %q — score %.0f (us) vs %.0f (them)\n", s.War.OpponentName, s.War.OurScore, s.War.TheirScore)
	} else {
		b.WriteString("  Not currently at war.\n")
	}
	if s.War.CompletedWars == 0 {
		b.WriteString("  No completed wars yet.\n")
	} else {
		fmt.Fprintf(&b, "  %d completed wars: %d wins, %d losses\n", s.War.CompletedWars, s.War.Wins, s.War.Losses)
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "recruitment_calls": [{"username": "...", "recommendation": "accept|reject|undecided", "reason": "..."}], "war_insight": "...", "recommended_focus": "...", "notes": "..."}`)

	return b.String()
}

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the same way every other Phase B-F package does.
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
	b.WriteString("🏴 AI GUILD ASSISTANT\n\n")

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

	if len(rec.RecruitmentCalls) > 0 {
		b.WriteString("RECRUITMENT CALLS:\n")
		for _, call := range rec.RecruitmentCalls {
			icon := "❔"
			switch call.Recommendation {
			case "accept":
				icon = "✅"
			case "reject":
				icon = "❌"
			}
			fmt.Fprintf(&b, "%s @%s — %s\n   💭 %s\n", icon, call.Username, call.Recommendation, call.Reason)
		}
		b.WriteString("\n")
	}

	if rec.WarInsight != "" {
		fmt.Fprintf(&b, "⚔️ War insight: %s\n", rec.WarInsight)
	}
	if rec.RecommendedFocus != "" {
		fmt.Fprintf(&b, "🎯 Recommended focus: %s\n", rec.RecommendedFocus)
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", rec.Notes)
	}

	b.WriteString("\nThis is a recommendation only — no applicant has been accepted/rejected and no war has been declared automatically.")
	return b.String()
}
