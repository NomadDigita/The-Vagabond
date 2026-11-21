package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/battleanalyst"
	"gopkg.in/telebot.v3"
)

type BattleAnalystHandler struct {
	Analyst *battleanalyst.Analyst
}

func NewBattleAnalystHandler(a *battleanalyst.Analyst) *BattleAnalystHandler {
	return &BattleAnalystHandler{Analyst: a}
}

func buildBattleAnalystKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Analysis", "battle_analyst_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func (h *BattleAnalystHandler) renderBattleAnalystReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Analyst.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return battleanalyst.FormatForTelegram(rec), buildBattleAnalystKeyboard(), nil
}

func (h *BattleAnalystHandler) HandleBattleAnalyst(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderBattleAnalystReport(ctx, sender.ID)
	if errors.Is(err, battleanalyst.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.", keyboards.AdvisorsNavigation())
	}
	if err != nil {
		return c.Send("⚠️ The AI Battle Analyst is temporarily unavailable: "+err.Error(), keyboards.AdvisorsNavigation())
	}

	return c.Send(text, telebot.ModeHTML, keyboard, keyboards.AdvisorsNavigation())
}

func (h *BattleAnalystHandler) HandleBattleAnalystRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderBattleAnalystReport(ctx, sender.ID)
	if errors.Is(err, battleanalyst.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Battle Analyst unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Analysis refreshed."})
	return c.Send(text, telebot.ModeHTML, keyboard)
}
