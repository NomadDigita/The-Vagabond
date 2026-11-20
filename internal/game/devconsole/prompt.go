package devconsole

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type NewPlayer struct {
	Username      string
	FirstName     string
	JoinedAt      string
	HomeContinent string
}

type TopPlayer struct {
	Name  string
	Score float64
	Level int
}

type Snapshot struct {
	WindowDays int

	NewPlayers        []NewPlayer
	NewPlayerCount    int
	TotalUsersAllTime int

	TopPlayers []TopPlayer

	ActiveUserCount int

	RecentWorldNews []string
}

type Recommendation struct {
	Summary                  string   `json:"summary"`
	Highlights               []string `json:"highlights"`
	NewPlayerNarrative       string   `json:"new_player_narrative"`
	TopPerformerNarrative    string   `json:"top_performer_narrative"`
	RecommendationsForAdmins string   `json:"recommendations_for_admins"`
	Notes                    string   `json:"notes"`

	Snapshot *Snapshot `json:"-"`

	FellBackToRawText bool
	Truncated         bool
}

const SystemPrompt = `You are the AI Developer Console for The Vagabond, a tick-based multiplayer survival/strategy game. You produce a game-activity summary for the game's ADMINISTRATORS, not for players.

Your job: summarize new player signups, top players, and overall activity for the given reporting window, grounded strictly in the numbers provided. You NEVER take any action yourself — you only report and observe.

Rules:
- Ground every claim in the actual numbers given — never invent a trend, spike, or concern the data doesn't support.
- If NewPlayerCount is 0, say so plainly rather than fabricating signup activity.
- If NewPlayerCount is larger than the number of NewPlayers actually listed (the list is capped for length), acknowledge you're highlighting a sample, not the full list.
- A player with no HomeContinent registered but never finished onboarding (no outpost yet) — mention this distinctly from an active new player if it's a meaningful fraction of new signups, since it may indicate an onboarding friction point worth flagging to admins.
- recommendations_for_admins should be concrete and grounded (e.g. "onboarding drop-off is high this window — worth checking the first-run flow") — if nothing in the data warrants a recommendation, say so honestly rather than inventing one.
- Keep summary to 2-3 sentences.`

func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "REPORTING WINDOW: last %d day(s)\n", s.WindowDays)
	fmt.Fprintf(&b, "Total users all-time: %d | Active in window (last_active): %d\n\n", s.TotalUsersAllTime, s.ActiveUserCount)

	fmt.Fprintf(&b, "NEW PLAYERS THIS WINDOW: %d total\n", s.NewPlayerCount)
	if len(s.NewPlayers) == 0 {
		b.WriteString("  None.\n")
	} else {
		if s.NewPlayerCount > len(s.NewPlayers) {
			fmt.Fprintf(&b, "  (showing %d most recent of %d total)\n", len(s.NewPlayers), s.NewPlayerCount)
		}
		for _, p := range s.NewPlayers {
			name := p.FirstName
			if p.Username != "" {
				name = fmt.Sprintf("%s (@%s)", name, p.Username)
			}
			continent := p.HomeContinent
			if continent == "" {
				continent = "no outpost yet"
			}
			fmt.Fprintf(&b, "  - %s, joined %s, home: %s\n", name, p.JoinedAt, continent)
		}
	}

	b.WriteString("\nTOP PLAYERS (all-time ranking):\n")
	if len(s.TopPlayers) == 0 {
		b.WriteString("  None.\n")
	} else {
		for i, tp := range s.TopPlayers {
			fmt.Fprintf(&b, "  %d. %s — Level %d, Score %.0f\n", i+1, tp.Name, tp.Level, tp.Score)
		}
	}

	b.WriteString("\nRECENT WORLD NEWS THIS WINDOW:\n")
	if len(s.RecentWorldNews) == 0 {
		b.WriteString("  None.\n")
	} else {
		for _, headline := range s.RecentWorldNews {
			fmt.Fprintf(&b, "  - %s\n", headline)
		}
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "highlights": ["...", "..."], "new_player_narrative": "...", "top_performer_narrative": "...", "recommendations_for_admins": "...", "notes": "..."}`)

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
	b.WriteString("🖥️ " + ai.HTMLBold("AI DEVELOPER CONSOLE — GAME ACTIVITY REPORT") + "\n\n")

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

	if len(rec.Highlights) > 0 {
		b.WriteString("✨ " + ai.HTMLBold("HIGHLIGHTS") + "\n")
		for _, h := range rec.Highlights {
			fmt.Fprintf(&b, "  • %s\n", ai.HTMLEscape(h))
		}
		b.WriteString("\n")
	}

	if rec.Snapshot != nil && len(rec.Snapshot.TopPlayers) > 0 {
		headers := []string{"#", "Player", "Lvl", "Score"}
		rows := make([][]string, 0, len(rec.Snapshot.TopPlayers))
		for i, tp := range rec.Snapshot.TopPlayers {
			name := tp.Name
			if r := []rune(name); len(r) > 18 {
				name = string(r[:17]) + "…"
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", i+1),
				ai.HTMLEscape(name),
				fmt.Sprintf("%d", tp.Level),
				fmt.Sprintf("%.0f", tp.Score),
			})
		}
		b.WriteString("🏆 " + ai.HTMLBold("TOP PLAYERS") + "\n")
		b.WriteString(ai.HTMLTable(headers, rows) + "\n\n")
	}

	if rec.NewPlayerNarrative != "" {
		b.WriteString("🆕 " + ai.HTMLBold("New players") + "\n")
		b.WriteString(ai.HTMLExpandableQuote(ai.HTMLEscape(rec.NewPlayerNarrative)) + "\n\n")
	}
	if rec.TopPerformerNarrative != "" {
		b.WriteString("👑 " + ai.HTMLBold("Top performers") + "\n")
		b.WriteString(ai.HTMLExpandableQuote(ai.HTMLEscape(rec.TopPerformerNarrative)) + "\n\n")
	}
	if rec.RecommendationsForAdmins != "" {
		b.WriteString("🛠️ " + ai.HTMLBold("For admins") + "\n")
		b.WriteString(ai.HTMLExpandableQuote(ai.HTMLEscape(rec.RecommendationsForAdmins)) + "\n\n")
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", ai.HTMLEscape(rec.Notes))
	}

	b.WriteString("\n" + ai.HTMLItalic("This is a read-only summary — no player, setting, or game data has been changed automatically."))
	return b.String()
}
