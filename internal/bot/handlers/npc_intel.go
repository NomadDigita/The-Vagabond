package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/npcintel"
	"gopkg.in/telebot.v3"
)

type NPCIntelHandler struct {
	Intel *npcintel.Intel
}

func NewNPCIntelHandler(in *npcintel.Intel) *NPCIntelHandler {
	return &NPCIntelHandler{Intel: in}
}

func buildNPCIntelKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Intel", "npc_intel_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func (h *NPCIntelHandler) renderNPCIntelReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Intel.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return npcintel.FormatForTelegram(rec), buildNPCIntelKeyboard(), nil
}

func (h *NPCIntelHandler) HandleNPCIntel(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderNPCIntelReport(ctx, sender.ID)
	if errors.Is(err, npcintel.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.", keyboards.AdvisorsNavigation())
	}
	if err != nil {
		return c.Send("⚠️ The AI NPC Intelligence advisor is temporarily unavailable: "+err.Error(), keyboards.AdvisorsNavigation())
	}

	return c.Send(text, telebot.ModeHTML, keyboard, keyboards.AdvisorsNavigation())
}

func (h *NPCIntelHandler) HandleNPCIntelRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderNPCIntelReport(ctx, sender.ID)
	if errors.Is(err, npcintel.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ NPC Intelligence unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Intel refreshed."})
	return c.Send(text, telebot.ModeHTML, keyboard)
}
