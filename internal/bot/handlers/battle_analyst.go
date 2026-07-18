package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/game/battleanalyst"
	"gopkg.in/telebot.v3"
)

// BattleAnalystHandler exposes Phase F (AI Battle Analyst). New
// command name (/battle_analyst) — no collision with any SpaceHunt
// phase 1-6 command or existing AI-roadmap commands.
type BattleAnalystHandler struct {
	Analyst *battleanalyst.Analyst
}

func NewBattleAnalystHandler(a *battleanalyst.Analyst) *BattleAnalystHandler {
	return &BattleAnalystHandler{Analyst: a}
}

// buildBattleAnalystKeyboard renders the inline keyboard attached to
// every Battle Analyst report: a single refresh button. Unlike
// Research Planner, there's no goal selection here — the analysis
// covers the player's whole combat record, not one steerable goal.
func buildBattleAnalystKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Analysis", "battle_analyst_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

// renderBattleAnalystReport runs a fresh analysis and returns the
// formatted text plus its attached keyboard, shared by the
// /battle_analyst command and its refresh callback so the two can
// never drift apart.
func (h *BattleAnalystHandler) renderBattleAnalystReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Analyst.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return battleanalyst.FormatForTelegram(rec), buildBattleAnalystKeyboard(), nil
}

// ── /battle_analyst ──────────────────────────────────────────────────
//
// Analyzes the player's accumulated raid (attacker + defender) and
// arena battle record, surfacing recurring patterns. Read-only: never
// changes any raid, arena battle, or unit on the player's behalf.
func (h *BattleAnalystHandler) HandleBattleAnalyst(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderBattleAnalystReport(ctx, sender.ID)
	if errors.Is(err, battleanalyst.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Battle Analyst is temporarily unavailable: " + err.Error())
	}

	return c.Send(text, keyboard)
}

// ── callback: battle_analyst_refresh ─────────────────────────────────
//
// Re-runs the analysis on demand (a real new AI Foundation call,
// subject to the usual cost/cache/budget rules) and posts a fresh
// report.
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
	return c.Send(text, keyboard)
}
