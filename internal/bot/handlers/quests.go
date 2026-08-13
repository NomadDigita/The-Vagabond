package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/progression"
	"gopkg.in/telebot.v3"
)

// QuestsHandler is the panel/claim side of
// NEXT_GENERATION_FEATURES_PLAN.md Phase 2 - progress itself is
// tracked automatically via internal/game/progression.RecordEvent,
// called from every real action handler at its own completion point.
// This handler renders the board and handles the Claim button; all
// the actual claim logic lives in progression.ClaimQuest so both the
// panel and a future non-Telegram surface (if one's ever added) share
// identical rules.
type QuestsHandler struct {
	DB *sql.DB
}

func NewQuestsHandler(db *sql.DB) *QuestsHandler {
	return &QuestsHandler{DB: db}
}

// HandleQuestsPanel (/quests) shows today's dailies and this week's
// weeklies with a progress bar and a Claim button on anything complete
// and not yet claimed.
func (h *QuestsHandler) HandleQuestsPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return nil
	}

	panelText := "📋 " + htmlBold("QUEST BOARD") + "\n" + divider + "\n\n"
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	renderSection := func(label string, period progression.QuestPeriod) {
		panelText += htmlBold(label) + "\n"
		any := false
		for _, q := range progression.Quests {
			if q.Period != period {
				continue
			}
			any = true
			var progressVal int
			var completedAt, claimedAt sql.NullTime
			_ = h.DB.QueryRowContext(ctx, `
				SELECT progress, completed_at, claimed_at FROM player_quest_progress
				WHERE user_id = $1 AND quest_key = $2 AND period_start = $3`,
				sender.ID, q.Key, questPeriodStartForPanel(period)).Scan(&progressVal, &completedAt, &claimedAt)

			status := fmt.Sprintf("%d/%d", progressVal, q.GoalAmount)
			if claimedAt.Valid {
				panelText += fmt.Sprintf("✅ %s (%s) - claimed\n", htmlEscape(q.Description), htmlCode(status))
			} else if completedAt.Valid {
				panelText += fmt.Sprintf("🎁 %s (%s) - ready to claim! +%s\n", htmlEscape(q.Description), htmlCode(status), htmlCode(fmt.Sprintf("$%.0f", q.RewardDollars)))
				buttons = append(buttons, selector.Row(selector.Data(fmt.Sprintf("🎁 Claim: %s", q.Description), "quest_claim", q.Key)))
			} else {
				panelText += fmt.Sprintf("⏳ %s (%s) - +%s\n", htmlEscape(q.Description), htmlCode(status), htmlCode(fmt.Sprintf("$%.0f", q.RewardDollars)))
			}
		}
		if !any {
			panelText += htmlItalic("Nothing here right now.") + "\n"
		}
		panelText += "\n"
	}

	renderSection("☀️ Daily (resets at UTC midnight)", progression.PeriodDaily)
	renderSection("📅 Weekly (resets Monday UTC)", progression.PeriodWeekly)

	panelText += divider
	selector.Inline(buttons...)

	return sendPanelWithNavHTML(c, navCaptionMain, keyboards.MainNavigation(), panelText, selector)
}

// questPeriodStartForPanel mirrors progression's own periodStart via
// its exported PeriodStart wrapper - kept as a tiny local helper so
// the query call site below reads clearly without importing "time"
// just for this one lookup.
func questPeriodStartForPanel(period progression.QuestPeriod) time.Time {
	return progression.PeriodStart(period)
}

// HandleQuestClaimCallback fires when a player taps a Claim button on
// HandleQuestsPanel.
func (h *QuestsHandler) HandleQuestClaimCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid claim request."})
	}
	questKey := c.Args()[0]

	if err := progression.ClaimQuest(ctx, h.DB, sender.ID, questKey); err != nil {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "❌ Couldn't claim that quest - it may not be complete yet, or was already claimed."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🎁 Reward claimed!"})
	return h.HandleQuestsPanel(c)
}
