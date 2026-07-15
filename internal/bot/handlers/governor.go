package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/game/governor"
	"gopkg.in/telebot.v3"
)

// GovernorHandler exposes Phase B (AI Planet Governor) to players.
// Command names (/governor, /governor_autopilot) are new and cannot
// collide with any SpaceHunt phase 1-6 command. Callback unique names
// (gov_refresh, gov_toggle_autopilot) follow the same convention as
// existing inline buttons in internal/bot/handlers/combat.go.
type GovernorHandler struct {
	Governor *governor.Governor
}

func NewGovernorHandler(g *governor.Governor) *GovernorHandler {
	return &GovernorHandler{Governor: g}
}

// buildGovernorKeyboard renders the inline keyboard attached to every
// Governor report: a refresh button, and an autopilot toggle whose
// label reflects the player's current stored preference.
func (h *GovernorHandler) buildGovernorKeyboard(ctx context.Context, userID int64) *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	autopilotOn, _ := h.Governor.AutopilotSetting(ctx, userID) // best-effort; defaults to OFF-label on error

	toggleLabel := "🛠️ Autopilot: OFF (tap to enable preference)"
	if autopilotOn {
		toggleLabel = "🛠️ Autopilot: ON (tap to disable preference)"
	}

	btnRefresh := selector.Data("🔄 Refresh Analysis", "gov_refresh")
	btnToggle := selector.Data(toggleLabel, "gov_toggle_autopilot")
	selector.Inline(selector.Row(btnRefresh), selector.Row(btnToggle))
	return selector
}

// renderGovernorReport runs a fresh recommendation and returns the
// formatted text plus its attached keyboard, shared by the /governor
// command and the refresh callback so the two can never drift apart.
func (h *GovernorHandler) renderGovernorReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Governor.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return governor.FormatForTelegram(rec), h.buildGovernorKeyboard(ctx, userID), nil
}

// ── /governor ────────────────────────────────────────────────────────
//
// Fetches the player's current base state, asks the AI Foundation for
// a recommendation, and shows it with a refresh + autopilot-preference
// keyboard. Read-only: never builds, upgrades, or queues anything on
// the player's behalf.
func (h *GovernorHandler) HandleGovernor(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderGovernorReport(ctx, sender.ID)
	if errors.Is(err, governor.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Planet Governor is temporarily unavailable: " + err.Error())
	}

	return c.Send(text, keyboard)
}

// ── callback: gov_refresh ────────────────────────────────────────────
//
// Re-runs the same analysis on demand and posts a fresh report. This
// makes a new AI Foundation call each tap (subject to the same
// cost/cache/budget rules as any other call — see internal/ai.Service),
// so it is a real re-analysis, not a cached replay of the same text.
func (h *GovernorHandler) HandleGovernorRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderGovernorReport(ctx, sender.ID)
	if errors.Is(err, governor.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Governor unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Analysis refreshed."})
	return c.Send(text, keyboard)
}

// ── callback: gov_toggle_autopilot ──────────────────────────────────
//
// Flips the player's stored autopilot preference and re-sends the
// keyboard with an updated label. Per governor.SetAutopilot's doc
// comment, this preference is not currently acted upon by any code
// path — the callback response says so explicitly so the tap can
// never be mistaken for enabling real automation.
func (h *GovernorHandler) HandleGovernorToggleCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	current, err := h.Governor.AutopilotSetting(ctx, sender.ID)
	if err != nil && !errors.Is(err, governor.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not read preference: " + err.Error()})
	}
	newState := !current
	if err := h.Governor.SetAutopilot(ctx, sender.ID, newState); err != nil {
		if errors.Is(err, governor.ErrNoEncampment) {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
		}
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to save: " + err.Error()})
	}

	stateWord := "OFF"
	if newState {
		stateWord = "ON"
	}
	_ = c.Respond(&telebot.CallbackResponse{Text: "✅ Autopilot preference: " + stateWord})
	return c.Send(fmt.Sprintf(
		"✅ Autopilot preference saved as %s.\n\nReminder: autopilot execution is not implemented yet (tracked in PROJECT_MASTER_PLAN.md) — /governor remains advisory-only for now regardless of this setting.",
		stateWord,
	), h.buildGovernorKeyboard(ctx, sender.ID))
}

// ── /governor_autopilot <on|off> ─────────────────────────────────────
//
// Text-command equivalent of the toggle button, for players who prefer
// typing a command over tapping. Intentionally does NOT cause any
// autonomous action yet (see PROJECT_MASTER_PLAN.md §4).
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
		return c.Send("Your autopilot preference is currently: "+state+"\nUsage: /governor_autopilot <on|off>\n\n"+
			"Note: autopilot execution is not implemented yet — the Governor is advisory-only for every player regardless of this setting. This just records your preference for when execution ships.",
			h.buildGovernorKeyboard(ctx, sender.ID))
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
	return c.Send("✅ Autopilot preference saved as "+stateWord+".\n\n"+
		"Reminder: autopilot execution is not implemented yet (tracked in PROJECT_MASTER_PLAN.md) — /governor remains advisory-only for now regardless of this setting.",
		h.buildGovernorKeyboard(ctx, sender.ID))
}
