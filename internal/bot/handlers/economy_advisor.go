package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/game/econadvisor"
	"gopkg.in/telebot.v3"
)

// EconomyAdvisorHandler exposes Phase D (AI Economy Advisor). New
// command name (/economy_advisor) — no collision with any SpaceHunt
// phase 1-6 command.
type EconomyAdvisorHandler struct {
	Advisor *econadvisor.Advisor
}

func NewEconomyAdvisorHandler(advisor *econadvisor.Advisor) *EconomyAdvisorHandler {
	return &EconomyAdvisorHandler{Advisor: advisor}
}

func buildEconomyAdvisorKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Analysis", "econ_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func (h *EconomyAdvisorHandler) renderReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Advisor.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return econadvisor.FormatForTelegram(rec), buildEconomyAdvisorKeyboard(), nil
}

// ── /economy_advisor ──────────────────────────────────────────────────
//
// Analyzes the player's resources, buildings, bank debt, and market
// activity, and recommends the highest-ROI next actions with
// quantitative expected gains. Read-only: never buys, sells, or
// upgrades anything on the player's behalf.
func (h *EconomyAdvisorHandler) HandleEconomyAdvisor(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderReport(ctx, sender.ID)
	if errors.Is(err, econadvisor.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Economy Advisor is temporarily unavailable: " + err.Error())
	}

	return c.Send(text, keyboard)
}

// ── callback: econ_refresh ────────────────────────────────────────────
//
// Re-runs the same analysis on demand (a real new AI Foundation call,
// subject to the usual cost/cache/budget rules) and posts a fresh report.
func (h *EconomyAdvisorHandler) HandleEconomyAdvisorRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderReport(ctx, sender.ID)
	if errors.Is(err, econadvisor.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Economy Advisor unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Analysis refreshed."})
	return c.Send(text, keyboard)
}
