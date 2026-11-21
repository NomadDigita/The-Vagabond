package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/guildassistant"
	"gopkg.in/telebot.v3"
)

type GuildAssistantHandler struct {
	Assistant *guildassistant.Assistant
}

func NewGuildAssistantHandler(a *guildassistant.Assistant) *GuildAssistantHandler {
	return &GuildAssistantHandler{Assistant: a}
}

func buildGuildAssistantKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Analysis", "guild_assistant_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func (h *GuildAssistantHandler) renderGuildAssistantReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Assistant.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return guildassistant.FormatForTelegram(rec), buildGuildAssistantKeyboard(), nil
}

func errorMessageForGuildAssistant(err error) string {
	switch {
	case errors.Is(err, guildassistant.ErrNoClan):
		return "❌ You're not in a Clan yet. Use the Clan panel to join or create one first."
	case errors.Is(err, guildassistant.ErrNotLeader):
		return "❌ Access Denied: the AI Guild Assistant is for Clan Leaders only."
	default:
		return "⚠️ The AI Guild Assistant is temporarily unavailable: " + err.Error()
	}
}

func (h *GuildAssistantHandler) HandleGuildAssistant(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderGuildAssistantReport(ctx, sender.ID)
	if err != nil {
		return c.Send(errorMessageForGuildAssistant(err), keyboards.AdvisorsNavigation())
	}

	return c.Send(text, telebot.ModeHTML, keyboard, keyboards.AdvisorsNavigation())
}

func (h *GuildAssistantHandler) HandleGuildAssistantRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderGuildAssistantReport(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: errorMessageForGuildAssistant(err)})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Analysis refreshed."})
	return c.Send(text, telebot.ModeHTML, keyboard)
}
