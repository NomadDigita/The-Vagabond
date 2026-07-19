// Package devconsole implements Phase J of the AI Systems Roadmap: the
// AI Developer Console. Like every other Phase B-I package, this is
// the seam where game state meets ai.Service; internal/ai itself
// stays game-agnostic per ADR-001.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in console.go.
//
// Scope, set explicitly by the project owner rather than guessed at
// (see ADR-019 in PROJECT_MASTER_PLAN.md): Phase J's name is the
// vaguest on the roadmap and, unlike Phases F-I, wasn't grounded in an
// already-built player-facing system. Rather than invent scope, this
// session asked and was given a concrete brief: an admin-only AI
// summary of the last N days' game activity — new player signups,
// top players, and similar. This package covers exactly that; it is
// NOT a general natural-language admin query tool, a content/balance
// suggestion engine, or anything else "developer console" might
// otherwise suggest — those remain unbuilt unless a future session is
// asked for them specifically.
//
// Honest data-availability note: the game collects no real-world
// IP/geolocation data for any user (users has no such column — see
// cmd/bot/main.go's CREATE TABLE). "Where a new player is from" is
// reported here as their in-game home continent (via
// encampments -> coordinates.region, the same Africa/Europe/Asia/
// Americas quadrant scheme every other world-aware phase already
// uses), not a real-world location. A player who registered but never
// finished onboarding (no encampment yet) has no home continent to
// report — that's shown honestly rather than guessed at.
package devconsole

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// NewPlayer is one user who registered within the reporting window.
type NewPlayer struct {
	Username      string // may be "" — Telegram username is optional
	FirstName     string
	JoinedAt      string // pre-formatted (e.g. "2026-07-15 14:32 UTC") — see console.go
	HomeContinent string // "" if the player never finished onboarding (no encampment yet)
}

// TopPlayer is one entry from the existing Global Ranking (see
// internal/game/scoring.ScoreExpr — the same formula
// HandleRankingPanel already uses, so this never disagrees with what
// players see on that screen).
type TopPlayer struct {
	Name  string
	Score float64
	Level int
}

// Snapshot is everything about the reporting window fed to the model.
type Snapshot struct {
	WindowDays int

	NewPlayers        []NewPlayer // capped — see console.go's newPlayerListCap
	NewPlayerCount    int         // true total, even if NewPlayers was capped for prompt size
	TotalUsersAllTime int

	TopPlayers []TopPlayer

	ActiveUserCount int // users with last_active within the window

	RecentWorldNews []string // most-recent-first, capped — see console.go
}

// Recommendation is the Developer Console's structured output for a
// weekly (or other window) game activity report.
type Recommendation struct {
	Summary                  string   `json:"summary"`
	Highlights               []string `json:"highlights"`
	NewPlayerNarrative       string   `json:"new_player_narrative"`
	TopPerformerNarrative    string   `json:"top_performer_narrative"`
	RecommendationsForAdmins string   `json:"recommendations_for_admins"`
	Notes                    string   `json:"notes"`

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
// Developer Console report.
const SystemPrompt = `You are the AI Developer Console for The Vagabond, a tick-based multiplayer survival/strategy game. You produce a game-activity summary for the game's ADMINISTRATORS, not for players.

Your job: summarize new player signups, top players, and overall activity for the given reporting window, grounded strictly in the numbers provided. You NEVER take any action yourself — you only report and observe.

Rules:
- Ground every claim in the actual numbers given — never invent a trend, spike, or concern the data doesn't support.
- If NewPlayerCount is 0, say so plainly rather than fabricating signup activity.
- If NewPlayerCount is larger than the number of NewPlayers actually listed (the list is capped for length), acknowledge you're highlighting a sample, not the full list.
- A player with no HomeContinent registered but never finished onboarding (no outpost yet) — mention this distinctly from an active new player if it's a meaningful fraction of new signups, since it may indicate an onboarding friction point worth flagging to admins.
- recommendations_for_admins should be concrete and grounded (e.g. "onboarding drop-off is high this window — worth checking the first-run flow") — if nothing in the data warrants a recommendation, say so honestly rather than inventing one.
- Keep summary to 2-3 sentences.`

// BuildUserPrompt renders a Snapshot into the data block the model
// reasons over.
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

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the same way every other Phase B-I package does.
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
	b.WriteString("🖥️ AI DEVELOPER CONSOLE — GAME ACTIVITY REPORT\n\n")

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

	if len(rec.Highlights) > 0 {
		b.WriteString("HIGHLIGHTS:\n")
		for _, h := range rec.Highlights {
			fmt.Fprintf(&b, "  • %s\n", h)
		}
		b.WriteString("\n")
	}

	if rec.NewPlayerNarrative != "" {
		fmt.Fprintf(&b, "🆕 New players: %s\n\n", rec.NewPlayerNarrative)
	}
	if rec.TopPerformerNarrative != "" {
		fmt.Fprintf(&b, "👑 Top performers: %s\n\n", rec.TopPerformerNarrative)
	}
	if rec.RecommendationsForAdmins != "" {
		fmt.Fprintf(&b, "🛠️ For admins: %s\n\n", rec.RecommendationsForAdmins)
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", rec.Notes)
	}

	b.WriteString("\nThis is a read-only summary — no player, setting, or game data has been changed automatically.")
	return b.String()
}
