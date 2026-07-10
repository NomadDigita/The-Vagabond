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

func (h *RankingHandler) HandleRankingPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🏆━━━━━━━━━━━━━━━━━━━━━━🏆\n" +
		"🌍 GLOBAL WASTELAND RANKING 🌍\n" +
		"🏆━━━━━━━━━━━━━━━━━━━━━━🏆\n\n"

	// ── Top Players ────────────────────────────────────────────────
	panelText += "👑 TOP SURVIVORS:\n"
	topPlayersQuery := fmt.Sprintf(`
		SELECT e.name, %s AS score
		FROM encampments e
		ORDER BY score DESC
		LIMIT 15`, scoring.ScoreExpr)

	rows, err := h.DB.QueryContext(ctx, topPlayersQuery)
	if err == nil {
		rank := 1
		for rows.Next() {
			var name string
			var score float64
			if scanErr := rows.Scan(&name, &score); scanErr == nil {
				panelText += fmt.Sprintf("%s %d. %s — 🏅 %.0f pts\n", medalFor(rank), rank, name, score)
				rank++
			}
		}
		rows.Close()
	}

	// ── Top Skilled (military-only) ───────────────────────────────
	panelText += "\n⚔️ TOP SKILLED (Military Might):\n"
	topSkilledQuery := fmt.Sprintf(`
		SELECT e.name, %s AS mil_score
		FROM encampments e
		ORDER BY mil_score DESC
		LIMIT 5`, scoring.MilitaryScoreExpr)

	rows2, err := h.DB.QueryContext(ctx, topSkilledQuery)
	if err == nil {
		rank := 1
		for rows2.Next() {
			var name string
			var score float64
			if scanErr := rows2.Scan(&name, &score); scanErr == nil {
				panelText += fmt.Sprintf("%s %d. %s — ⚔️ %.0f Combat Rating\n", medalFor(rank), rank, name, score)
				rank++
			}
		}
		rows2.Close()
	}

	// ── Top Guilds ──────────────────────────────────────────────────
	panelText += "\n🛡️ TOP GUILDS:\n"
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
				panelText += fmt.Sprintf("%s %d. 🏴 %s (%d/8) — 🏅 %.0f pts\n", medalFor(rank), rank, name, members, score)
				rank++
			}
		}
		rows3.Close()
	}

	panelText += "\n🏆━━━━━━━━━━━━━━━━━━━━━━🏆"

	return c.Send(panelText, keyboards.MainNavigation())
}
