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
	rec, err := h.Advisor.Recommend(ctx, sender.ID)
	if errors.Is(err, econadvisor.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Economy Advisor is temporarily unavailable: " + err.Error())
	}

	return c.Send(econadvisor.FormatForTelegram(rec))
}
