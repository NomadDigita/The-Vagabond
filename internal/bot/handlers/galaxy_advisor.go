package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/galaxyadvisor"
	"gopkg.in/telebot.v3"
)

type GalaxyAdvisorHandler struct {
	Advisor *galaxyadvisor.Advisor
}

func NewGalaxyAdvisorHandler(a *galaxyadvisor.Advisor) *GalaxyAdvisorHandler {
	return &GalaxyAdvisorHandler{Advisor: a}
}

func buildGalaxyAdvisorKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Briefing", "galaxy_advisor_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func (h *GalaxyAdvisorHandler) renderGalaxyAdvisorReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Advisor.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return galaxyadvisor.FormatForTelegram(rec), buildGalaxyAdvisorKeyboard(), nil
}

func (h *GalaxyAdvisorHandler) HandleGalaxyAdvisor(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderGalaxyAdvisorReport(ctx, sender.ID)
	if errors.Is(err, galaxyadvisor.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.", keyboards.AdvisorsNavigation())
	}
	if err != nil {
		return c.Send("⚠️ The AI Dynamic Galaxy advisor is temporarily unavailable: "+err.Error(), keyboards.AdvisorsNavigation())
	}

	return c.Send(text, telebot.ModeHTML, keyboard, keyboards.AdvisorsNavigation())
}

func (h *GalaxyAdvisorHandler) HandleGalaxyAdvisorRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderGalaxyAdvisorReport(ctx, sender.ID)
	if errors.Is(err, galaxyadvisor.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Galaxy advisor unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Briefing refreshed."})
	return c.Send(text, telebot.ModeHTML, keyboard)
}
