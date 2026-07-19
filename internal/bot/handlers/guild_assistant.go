package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/game/guildassistant"
	"gopkg.in/telebot.v3"
)

// GuildAssistantHandler exposes Phase G (AI Guild Assistant). New
// command (/guild_assistant) — no collision with any existing clan
// command in internal/bot/handlers/clan.go.
//
// Leader-only, unlike every prior Phase B-F command which any player
// can use for their own base/fleet/economy/research/battles: this
// feature reasons about clan-wide recruitment and war strategy, which
// clan.go already treats as Leader-gated decisions (see
// guildassistant/assistant.go's ErrNotLeader doc comment).
type GuildAssistantHandler struct {
	Assistant *guildassistant.Assistant
}

func NewGuildAssistantHandler(a *guildassistant.Assistant) *GuildAssistantHandler {
	return &GuildAssistantHandler{Assistant: a}
}

// buildGuildAssistantKeyboard renders the inline keyboard attached to
// every Guild Assistant report: a single refresh button. Like Battle
// Analyst, there's no goal selection here — the analysis covers
// whatever the clan's roster/applicants/war state currently is, not
// one steerable goal.
func buildGuildAssistantKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Analysis", "guild_assistant_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

// renderGuildAssistantReport runs a fresh analysis and returns the
// formatted text plus its attached keyboard, shared by the
// /guild_assistant command and its refresh callback so the two can
// never drift apart.
func (h *GuildAssistantHandler) renderGuildAssistantReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Assistant.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return guildassistant.FormatForTelegram(rec), buildGuildAssistantKeyboard(), nil
}

// errorMessageFor maps the Assistant's sentinel errors to a
// player-facing message, shared by the command and its refresh
// callback.
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

// ── /guild_assistant ─────────────────────────────────────────────────
//
// Analyzes the Leader's clan: pending recruitment applicants, roster
// health, and war record. Read-only: never accepts/rejects an
// applicant, declares a war, or changes membership on the Leader's
// behalf.
func (h *GuildAssistantHandler) HandleGuildAssistant(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderGuildAssistantReport(ctx, sender.ID)
	if err != nil {
		return c.Send(errorMessageForGuildAssistant(err))
	}

	return c.Send(text, keyboard)
}

// ── callback: guild_assistant_refresh ────────────────────────────────
//
// Re-runs the analysis on demand (a real new AI Foundation call,
// subject to the usual cost/cache/budget rules) and posts a fresh
// report.
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
	return c.Send(text, keyboard)
}
