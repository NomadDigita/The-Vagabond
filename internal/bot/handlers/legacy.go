package handlers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type LegacyHandler struct {
	DB *sql.DB
}

func NewLegacyHandler(db *sql.DB) *LegacyHandler {
	return &LegacyHandler{DB: db}
}

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

func truncateEpitaph(epitaph string, maxLen int) string {
	runes := []rune(epitaph)
	if len(runes) <= maxLen {
		return epitaph
	}
	return string(runes[:maxLen]) + "…"
}
