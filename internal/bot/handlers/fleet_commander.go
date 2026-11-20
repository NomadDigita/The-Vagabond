package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/fleetcommander"
	"gopkg.in/telebot.v3"
)

type FleetCommanderHandler struct {
	Commander *fleetcommander.Commander
}

func NewFleetCommanderHandler(cmd *fleetcommander.Commander) *FleetCommanderHandler {
	return &FleetCommanderHandler{Commander: cmd}
}

func buildFleetCommanderKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Analysis", "fleet_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func (h *FleetCommanderHandler) renderReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Commander.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return fleetcommander.FormatForTelegram(rec), buildFleetCommanderKeyboard(), nil
}

func (h *FleetCommanderHandler) HandleFleetCommander(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderReport(ctx, sender.ID)
	if errors.Is(err, fleetcommander.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.", keyboards.AdvisorsNavigation())
	}
	if err != nil {
		return c.Send("⚠️ The AI Fleet Commander is temporarily unavailable: "+err.Error(), keyboards.AdvisorsNavigation())
	}

	return c.Send(text, telebot.ModeHTML, keyboard, keyboards.AdvisorsNavigation())
}

func (h *FleetCommanderHandler) HandleFleetCommanderRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderReport(ctx, sender.ID)
	if errors.Is(err, fleetcommander.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Fleet Commander unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Analysis refreshed."})
	return c.Send(text, telebot.ModeHTML, keyboard)
}
