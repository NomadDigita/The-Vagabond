package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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

func displayName(name string, isAIFaction bool) string {
	if isAIFaction {
		return "🤖 " + htmlEscape(name)
	}
	return htmlEscape(name)
}

func rankTable(header string, rows [][3]string) string {
	if len(rows) == 0 {
		return htmlItalic("No entries yet.") + "\n"
	}
	var table strings.Builder
	table.WriteString(fmt.Sprintf("<table bordered striped>\n<tr><th></th><th>Name</th><th>%s</th></tr>\n", htmlEscape(header)))
	for _, r := range rows {
		table.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td></tr>\n", r[0], r[1], r[2]))
	}
	table.WriteString("</table>\n")
	return table.String()
}

func (h *RankingHandler) HandleRankingPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🏆 " + htmlBold("GLOBAL WASTELAND RANKING") + " 🏆\n" + divider + "\n\n"

	panelText += "👑 " + htmlBold("TOP SURVIVORS") + "\n"
	topPlayersQuery := fmt.Sprintf(`
		SELECT e.name, e.is_ai_faction, %s AS score
		FROM encampments e
		ORDER BY score DESC
		LIMIT 15`, scoring.ScoreExpr)

	var topPlayerRows [][3]string
	rows, err := h.DB.QueryContext(ctx, topPlayersQuery)
	if err == nil {
		rank := 1
		for rows.Next() {
			var name string
			var isAI bool
			var score float64
			if scanErr := rows.Scan(&name, &isAI, &score); scanErr == nil {
				topPlayerRows = append(topPlayerRows, [3]string{
					fmt.Sprintf("%s %d", medalFor(rank), rank),
					displayName(name, isAI),
					htmlCode(fmt.Sprintf("%.0f", score)),
				})
				rank++
			}
		}
		rows.Close()
	}
	panelText += rankTable("Score", topPlayerRows) + "\n"

	panelText += "⚔️ " + htmlBold("TOP SKILLED (Military Might)") + "\n"
	topSkilledQuery := fmt.Sprintf(`
		SELECT e.name, e.is_ai_faction, %s AS mil_score
		FROM encampments e
		ORDER BY mil_score DESC
		LIMIT 5`, scoring.MilitaryScoreExpr)

	var topSkilledRows [][3]string
	rows2, err := h.DB.QueryContext(ctx, topSkilledQuery)
	if err == nil {
		rank := 1
		for rows2.Next() {
			var name string
			var isAI bool
			var score float64
			if scanErr := rows2.Scan(&name, &isAI, &score); scanErr == nil {
				topSkilledRows = append(topSkilledRows, [3]string{
					fmt.Sprintf("%s %d", medalFor(rank), rank),
					displayName(name, isAI),
					htmlCode(fmt.Sprintf("%.0f", score)),
				})
				rank++
			}
		}
		rows2.Close()
	}
	panelText += rankTable("Combat Rating", topSkilledRows) + "\n"

	panelText += "🛡️ " + htmlBold("TOP GUILDS") + "\n"
	topGuildsQuery := fmt.Sprintf(`
		SELECT cl.id, cl.name, COUNT(uc.user_id) AS members, COALESCE(SUM(%s), 0) AS total_score
		FROM clans cl
		JOIN user_clans uc ON uc.clan_id = cl.id
		JOIN encampments e ON e.user_id = uc.user_id
		GROUP BY cl.id, cl.name
		ORDER BY total_score DESC
		LIMIT 10`, scoring.ScoreExpr)

	var topGuildRows [][3]string
	rows3, err := h.DB.QueryContext(ctx, topGuildsQuery)
	if err == nil {
		rank := 1
		for rows3.Next() {
			var clanID, name string
			var members int
			var score float64
			if scanErr := rows3.Scan(&clanID, &name, &members, &score); scanErr == nil {
				var banner string
				_ = h.DB.QueryRowContext(ctx, `
					SELECT cst.banner_emoji FROM constellation_members cm
					JOIN constellations cst ON cst.id = cm.constellation_id
					WHERE cm.clan_id = $1 AND cst.status = 'active' LIMIT 1`, clanID).Scan(&banner)
				nameCell := fmt.Sprintf("🏴 %s %s", htmlEscape(name), htmlCode(fmt.Sprintf("(%d/8)", members)))
				if banner != "" {
					nameCell += " " + banner
				}
				topGuildRows = append(topGuildRows, [3]string{
					fmt.Sprintf("%s %d", medalFor(rank), rank),
					nameCell,
					htmlCode(fmt.Sprintf("%.0f", score)),
				})
				rank++
			}
		}
		rows3.Close()
	}
	panelText += rankTable("Score", topGuildRows)

	panelText += "\n" + divider

	return sendRichMessage(c, panelText, keyboards.MainNavigation())
}
