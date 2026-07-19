package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/game/galaxyadvisor"
	"gopkg.in/telebot.v3"
)

// GalaxyAdvisorHandler exposes Phase H (AI Dynamic Galaxy). New
// command (/galaxy_advisor) — no collision with any existing world
// command (/world_feed, /sector_map, etc. in internal/bot/handlers/world.go).
type GalaxyAdvisorHandler struct {
	Advisor *galaxyadvisor.Advisor
}

func NewGalaxyAdvisorHandler(a *galaxyadvisor.Advisor) *GalaxyAdvisorHandler {
	return &GalaxyAdvisorHandler{Advisor: a}
}

// buildGalaxyAdvisorKeyboard renders the inline keyboard attached to
// every Galaxy Advisor briefing: a single refresh button. Like Battle
// Analyst and Guild Assistant, there's no goal selection — this
// briefing covers whatever the galaxy's current environmental state
// is, not one steerable goal.
func buildGalaxyAdvisorKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Briefing", "galaxy_advisor_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

// renderGalaxyAdvisorReport runs a fresh briefing and returns the
// formatted text plus its attached keyboard, shared by the
// /galaxy_advisor command and its refresh callback so the two can
// never drift apart.
func (h *GalaxyAdvisorHandler) renderGalaxyAdvisorReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Advisor.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return galaxyadvisor.FormatForTelegram(rec), buildGalaxyAdvisorKeyboard(), nil
}

// ── /galaxy_advisor ──────────────────────────────────────────────────
//
// Briefs the player on their home continent's current world event and
// the wider galaxy's environmental state. Read-only: never moves a
// fleet, queues construction, or changes any world event itself.
func (h *GalaxyAdvisorHandler) HandleGalaxyAdvisor(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderGalaxyAdvisorReport(ctx, sender.ID)
	if errors.Is(err, galaxyadvisor.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Dynamic Galaxy advisor is temporarily unavailable: " + err.Error())
	}

	return c.Send(text, keyboard)
}

// ── callback: galaxy_advisor_refresh ─────────────────────────────────
//
// Re-runs the briefing on demand (a real new AI Foundation call,
// subject to the usual cost/cache/budget rules) and posts a fresh
// report.
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
	return c.Send(text, keyboard)
}
