package handlers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
	"gopkg.in/telebot.v3"
)

type RankingHandler struct {
	DB *sql.DB
}

func NewRankingHandler(db *sql.DB) *RankingHandler {
	return &RankingHandler{DB: db}
}

// medalFor returns a rank-appropriate medal/trophy glyph, matching
// SpaceHunt's ranking board style.
func medalFor(rank int) string {
	switch rank {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return "🎖️"
	}
}

// displayName returns the escaped name for leaderboard display, prefixed
// with 🤖 for AI factions so parity in the score doesn't come at the cost
// of honest labeling in the text (AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md
// section 1.2).
func displayName(name string, isAIFaction bool) string {
	if isAIFaction {
		return "🤖 " + htmlEscape(name)
	}
	return htmlEscape(name)
}

func (h *RankingHandler) HandleRankingPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🏆 " + htmlBold("GLOBAL WASTELAND RANKING") + " 🏆\n" + divider + "\n\n"

	// ── Top Players ────────────────────────────────────────────────
	// AI factions are ranked identically to humans here (full parity -
	// they earned the score the same way), but are visually marked with
	// a 🤖 prefix so the distinction stays honest in the display line.
	// See AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.2.
	panelText += "👑 " + htmlBold("TOP SURVIVORS") + "\n"
	topPlayersQuery := fmt.Sprintf(`
		SELECT e.name, e.is_ai_faction, %s AS score
		FROM encampments e
		ORDER BY score DESC
		LIMIT 15`, scoring.ScoreExpr)

	rows, err := h.DB.QueryContext(ctx, topPlayersQuery)
	if err == nil {
		rank := 1
		for rows.Next() {
			var name string
			var isAI bool
			var score float64
			if scanErr := rows.Scan(&name, &isAI, &score); scanErr == nil {
				panelText += fmt.Sprintf("%s %d. %s — 🏅 %s pts\n", medalFor(rank), rank, displayName(name, isAI), htmlCode(fmt.Sprintf("%.0f", score)))
				rank++
			}
		}
		rows.Close()
	}

	// ── Top Skilled (military-only) ───────────────────────────────
	panelText += "\n⚔️ " + htmlBold("TOP SKILLED (Military Might)") + "\n"
	topSkilledQuery := fmt.Sprintf(`
		SELECT e.name, e.is_ai_faction, %s AS mil_score
		FROM encampments e
		ORDER BY mil_score DESC
		LIMIT 5`, scoring.MilitaryScoreExpr)

	rows2, err := h.DB.QueryContext(ctx, topSkilledQuery)
	if err == nil {
		rank := 1
		for rows2.Next() {
			var name string
			var isAI bool
			var score float64
			if scanErr := rows2.Scan(&name, &isAI, &score); scanErr == nil {
				panelText += fmt.Sprintf("%s %d. %s — ⚔️ %s Combat Rating\n", medalFor(rank), rank, displayName(name, isAI), htmlCode(fmt.Sprintf("%.0f", score)))
				rank++
			}
		}
		rows2.Close()
	}

	// ── Top Guilds ──────────────────────────────────────────────────
	panelText += "\n🛡️ " + htmlBold("TOP GUILDS") + "\n"
	topGuildsQuery := fmt.Sprintf(`
		SELECT cl.name, COUNT(uc.user_id) AS members, COALESCE(SUM(%s), 0) AS total_score
		FROM clans cl
		JOIN user_clans uc ON uc.clan_id = cl.id
		JOIN encampments e ON e.user_id = uc.user_id
		GROUP BY cl.name
		ORDER BY total_score DESC
		LIMIT 10`, scoring.ScoreExpr)

	rows3, err := h.DB.QueryContext(ctx, topGuildsQuery)
	if err == nil {
		rank := 1
		for rows3.Next() {
			var name string
			var members int
			var score float64
			if scanErr := rows3.Scan(&name, &members, &score); scanErr == nil {
				panelText += fmt.Sprintf("%s %d. 🏴 %s %s — 🏅 %s pts\n", medalFor(rank), rank, htmlEscape(name), htmlCode(fmt.Sprintf("(%d/8)", members)), htmlCode(fmt.Sprintf("%.0f", score)))
				rank++
			}
		}
		rows3.Close()
	}

	panelText += "\n" + divider

	return c.Send(panelText, telebot.ModeHTML, keyboards.MainNavigation())
}
