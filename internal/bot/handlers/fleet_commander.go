package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/game/fleetcommander"
	"gopkg.in/telebot.v3"
)

// FleetCommanderHandler exposes Phase C (AI Fleet Commander). New
// command name (/fleet_commander) — no collision with any SpaceHunt
// phase 1-6 command, and distinct from combat.go's existing
// HandleReconAICallback, which is a static templated report, not an
// LLM call.
type FleetCommanderHandler struct {
	Commander *fleetcommander.Commander
}

func NewFleetCommanderHandler(cmd *fleetcommander.Commander) *FleetCommanderHandler {
	return &FleetCommanderHandler{Commander: cmd}
}

// ── /fleet_commander ─────────────────────────────────────────────────
//
// Analyzes the player's fleet against the rogue-nest PvE target scaled
// to their level (see PROJECT_MASTER_PLAN.md — PvP target support is
// future work) plus their recent raid history, and returns one of:
// attack, retreat, reinforce, scout, wait, split_fleet — with
// reasoning. Read-only: never launches anything.
func (h *FleetCommanderHandler) HandleFleetCommander(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	rec, err := h.Commander.Recommend(ctx, sender.ID)
	if errors.Is(err, fleetcommander.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Fleet Commander is temporarily unavailable: " + err.Error())
	}

	return c.Send(fleetcommander.FormatForTelegram(rec))
}
