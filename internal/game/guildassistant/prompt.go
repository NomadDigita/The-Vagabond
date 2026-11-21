package guildassistant

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type Applicant struct {
	Username string
	Level    int
}

type WarRecord struct {
	CompletedWars int
	Wins          int
	Losses        int

	InActiveWar  bool
	OpponentName string
	OurScore     float64
	TheirScore   float64
}

type Snapshot struct {
	ClanID     string
	Name       string
	Recruiting bool

	MemberCount     int
	MemberCap       int
	CombinedLevel   int
	MilitaryPower   int
	InactiveMembers int

	PendingApplicants []Applicant
	War               WarRecord
}

type RecruitmentCall struct {
	Username       string `json:"username"`
	Recommendation string `json:"recommendation"`
	Reason         string `json:"reason"`
}

type Recommendation struct {
	Summary          string            `json:"summary"`
	RecruitmentCalls []RecruitmentCall `json:"recruitment_calls"`
	WarInsight       string            `json:"war_insight"`
	RecommendedFocus string            `json:"recommended_focus"`
	Notes            string            `json:"notes"`

	FellBackToRawText bool
	Truncated         bool
}

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
	b.WriteString("🏴 " + ai.HTMLBold("AI GUILD ASSISTANT") + "\n\n")

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

	if len(rec.RecruitmentCalls) > 0 {
		b.WriteString("🪖 " + ai.HTMLBold("RECRUITMENT CALLS") + "\n")
		for _, call := range rec.RecruitmentCalls {
			icon := "❔"
			switch call.Recommendation {
			case "accept":
				icon = "✅"
			case "reject":
				icon = "❌"
			}
			fmt.Fprintf(&b, "%s @%s — %s\n   💭 %s\n", icon, ai.HTMLEscape(call.Username), ai.HTMLEscape(call.Recommendation), ai.HTMLEscape(call.Reason))
		}
		b.WriteString("\n")
	}

	if rec.WarInsight != "" {
		fmt.Fprintf(&b, "⚔️ War insight: %s\n", ai.HTMLEscape(rec.WarInsight))
	}
	if rec.RecommendedFocus != "" {
		fmt.Fprintf(&b, "🎯 Recommended focus: %s\n", ai.HTMLEscape(rec.RecommendedFocus))
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", ai.HTMLEscape(rec.Notes))
	}

	b.WriteString("\n" + ai.HTMLItalic("This is a recommendation only — no applicant has been accepted/rejected and no war has been declared automatically."))
	return b.String()
}
