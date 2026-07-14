package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/game/governor"
	"gopkg.in/telebot.v3"
)

// GovernorHandler exposes Phase B (AI Planet Governor) to players.
// Command names (/governor, /governor_autopilot) are new and cannot
// collide with any SpaceHunt phase 1-6 command.
type GovernorHandler struct {
	Governor *governor.Governor
}

func NewGovernorHandler(g *governor.Governor) *GovernorHandler {
	return &GovernorHandler{Governor: g}
}

// ── /governor ────────────────────────────────────────────────────────
//
// Fetches the player's current base state, asks the AI Foundation for
// a recommendation, and shows it. Read-only: never builds, upgrades,
// or queues anything on the player's behalf.
func (h *GovernorHandler) HandleGovernor(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	rec, err := h.Governor.Recommend(ctx, sender.ID)
	if errors.Is(err, governor.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Planet Governor is temporarily unavailable: " + err.Error())
	}

	return c.Send(governor.FormatForTelegram(rec))
}

// ── /governor_autopilot <on|off> ─────────────────────────────────────
//
// Stores the player's autopilot preference. Intentionally does NOT
// cause any autonomous action yet (see PROJECT_MASTER_PLAN.md §4) —
// the reply is explicit about that so the setting can never be
// mistaken for something it isn't.
func (h *GovernorHandler) HandleGovernorAutopilot(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	payload := strings.ToLower(strings.TrimSpace(c.Message().Payload))
	if payload != "on" && payload != "off" {
		current, err := h.Governor.AutopilotSetting(ctx, sender.ID)
		state := "OFF"
		if current {
			state = "ON"
		}
		if err != nil && !errors.Is(err, governor.ErrNoEncampment) {
			return c.Send("⚠️ Could not read your current preference: " + err.Error())
		}
		return c.Send("Your autopilot preference is currently: " + state + "\nUsage: /governor_autopilot <on|off>\n\n" +
			"Note: autopilot execution is not implemented yet — the Governor is advisory-only for every player regardless of this setting. This just records your preference for when execution ships.")
	}

	enabled := payload == "on"
	if err := h.Governor.SetAutopilot(ctx, sender.ID, enabled); err != nil {
		if errors.Is(err, governor.ErrNoEncampment) {
			return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
		}
		return c.Send("⚠️ Failed to save preference: " + err.Error())
	}

	stateWord := "OFF"
	if enabled {
		stateWord = "ON"
	}
	return c.Send("✅ Autopilot preference saved as " + stateWord + ".\n\n" +
		"Reminder: autopilot execution is not implemented yet (tracked in PROJECT_MASTER_PLAN.md) — /governor remains advisory-only for now regardless of this setting.")
}
