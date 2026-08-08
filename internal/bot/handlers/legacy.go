package handlers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

// LegacyHandler is the panel side of RARE_WORLD_FEATURES_PLAN.md Phase
// 3 - the tick.Engine side (defeat detection, epitaph generation, pool
// exclusion) lives in internal/engine/tick/factiondefeat.go.
type LegacyHandler struct {
	DB *sql.DB
}

func NewLegacyHandler(db *sql.DB) *LegacyHandler {
	return &LegacyHandler{DB: db}
}

// HandleLegacyPanel (/legacy) shows the Hall of the Fallen - every
// permanently-defeated AI faction, most recent first, as a Rich
// Message <table> (name / defeated date / epitaph excerpt), matching
// the Hall of Relics (relicconvoy.go) and ranking.go's leaderboard
// table pattern. This is deliberately AI-faction-only content - see
// the plan doc's design principle 3: real players are never
// permanently defeated by anything in this feature set, so this panel
// will never show a real player's outpost.
func (h *LegacyHandler) HandleLegacyPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🪦 " + htmlBold("HALL OF THE FALLEN") + "\n" + divider + "\n\n" +
		htmlItalic("A permanent record of AI factions ground down to nothing - their banners are dust, but their story remains.") + "\n\n"

	rows, err := h.DB.QueryContext(ctx, `
		SELECT name, defeated_at, epitaph
		FROM encampments
		WHERE is_ai_faction = TRUE AND is_defeated = TRUE
		ORDER BY defeated_at DESC LIMIT 15`)
	if err != nil {
		panelText += "📡 " + htmlItalic("Static: Connection interrupted.") + "\n"
	} else {
		var hallRows [][3]string
		for rows.Next() {
			var name string
			var defeatedAt sql.NullTime
			var epitaph sql.NullString
			if scanErr := rows.Scan(&name, &defeatedAt, &epitaph); scanErr == nil {
				dateStr := "—"
				if defeatedAt.Valid {
					dateStr = defeatedAt.Time.Format("Jan 2")
				}
				excerpt := "—"
				if epitaph.Valid && epitaph.String != "" {
					excerpt = truncateEpitaph(epitaph.String, 60)
				}
				hallRows = append(hallRows, [3]string{fmt.Sprintf("🪦 %s", htmlEscape(name)), dateStr, htmlEscape(excerpt)})
			}
		}
		rows.Close()

		if len(hallRows) == 0 {
			panelText += htmlItalic("No faction has fallen yet - the wasteland's power balance remains unbroken. Keep raiding.") + "\n"
		} else {
			panelText += rankTable("Fell", hallRows)
		}
	}

	panelText += "\n" + divider

	selector := &telebot.ReplyMarkup{}
	selector.Inline()
	return sendPanelWithNavHTML(c, navCaptionMain, keyboards.MainNavigation(), panelText, selector)
}

// truncateEpitaph shortens a full epitaph to a table-friendly excerpt,
// appending an ellipsis when it had to cut - the full epitaph is still
// shown in full wherever it's the primary content (the exploration
// discovery message in engine.go's resolveExplorationDiscovery), this
// is only for the Hall of the Fallen's compact table column.
func truncateEpitaph(epitaph string, maxLen int) string {
	runes := []rune(epitaph)
	if len(runes) <= maxLen {
		return epitaph
	}
	return string(runes[:maxLen]) + "…"
}
