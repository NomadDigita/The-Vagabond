package handlers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/progression"
	"gopkg.in/telebot.v3"
)

// AchievementsHandler is the panel side of
// NEXT_GENERATION_FEATURES_PLAN.md Phase 1 - unlocking itself happens
// automatically via internal/game/progression.RecordEvent, called from
// every real action handler at its own completion point. This handler
// only ever reads player_achievements and player_event_counts to
// render progress; it never writes to them.
type AchievementsHandler struct {
	DB *sql.DB
}

func NewAchievementsHandler(db *sql.DB) *AchievementsHandler {
	return &AchievementsHandler{DB: db}
}

// HandleAchievementsPanel (/achievements) shows every achievement,
// grouped by category, as a Rich Message <table> per category -
// unlocked ones marked with a checkmark and their unlock date,
// in-progress ones showing "current/threshold" against the player's
// lifetime counter for that achievement's trigger.
func (h *AchievementsHandler) HandleAchievementsPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	unlocked := make(map[string]sql.NullTime)
	rows, err := h.DB.QueryContext(ctx, "SELECT achievement_key, unlocked_at FROM player_achievements WHERE user_id = $1", sender.ID)
	if err == nil {
		for rows.Next() {
			var key string
			var at sql.NullTime
			if scanErr := rows.Scan(&key, &at); scanErr == nil {
				unlocked[key] = at
			}
		}
		rows.Close()
	}

	counts := make(map[string]int)
	countRows, err := h.DB.QueryContext(ctx, "SELECT trigger, count FROM player_event_counts WHERE user_id = $1", sender.ID)
	if err == nil {
		for countRows.Next() {
			var trigger string
			var count int
			if scanErr := countRows.Scan(&trigger, &count); scanErr == nil {
				counts[trigger] = count
			}
		}
		countRows.Close()
	}

	unlockedCount := len(unlocked)
	totalCount := len(progression.Achievements)

	panelText := "🏆 " + htmlBold("ACHIEVEMENTS") + "\n" + divider + "\n" +
		fmt.Sprintf("%s unlocked\n\n", htmlCode(fmt.Sprintf("%d / %d", unlockedCount, totalCount)))

	categories := []struct{ key, label string }{
		{"combat", "⚔️ Combat"},
		{"economy", "💰 Economy"},
		{"exploration", "🔭 Exploration"},
		{"social", "🏴 Social"},
	}

	for _, cat := range categories {
		var catRows [][3]string
		for _, a := range progression.Achievements {
			if a.Category != cat.key {
				continue
			}
			if at, ok := unlocked[a.Key]; ok {
				dateStr := "—"
				if at.Valid {
					dateStr = at.Time.Format("Jan 2")
				}
				catRows = append(catRows, [3]string{
					fmt.Sprintf("✅ %s %s", a.Icon, htmlEscape(a.Name)),
					htmlEscape(a.Description),
					dateStr,
				})
			} else {
				progress := counts[a.Trigger]
				if progress > a.Threshold {
					progress = a.Threshold
				}
				catRows = append(catRows, [3]string{
					fmt.Sprintf("🔒 %s %s", a.Icon, htmlEscape(a.Name)),
					htmlEscape(a.Description),
					htmlCode(fmt.Sprintf("%d/%d", progress, a.Threshold)),
				})
			}
		}
		if len(catRows) == 0 {
			continue
		}
		panelText += htmlBold(cat.label) + "\n" + rankTable("Progress", catRows) + "\n"
	}

	panelText += divider

	selector := &telebot.ReplyMarkup{}
	selector.Inline()
	return sendPanelWithNavRich(c, navCaptionMain, keyboards.MainNavigation(), panelText, selector)
}
