package handlers

import (
	"context"
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/game/npcintel"
	"gopkg.in/telebot.v3"
)

// NPCIntelHandler exposes Phase I (AI NPC Intelligence). New command
// (/npc_intel) — distinct from /recon_ai (the static composition
// numbers, in combat.go) and /fleet_commander (the attack/no-attack
// call, Phase C) — this gives a composition-specific tactical read of
// the Rogue Drone Nest against the player's own fleet.
type NPCIntelHandler struct {
	Intel *npcintel.Intel
}

func NewNPCIntelHandler(in *npcintel.Intel) *NPCIntelHandler {
	return &NPCIntelHandler{Intel: in}
}

// buildNPCIntelKeyboard renders the inline keyboard attached to every
// NPC Intelligence report: a single refresh button. Like Battle
// Analyst, Guild Assistant, and Galaxy Advisor, there's no goal
// selection — there's only one Nest (scaled to the player's level) to
// read.
func buildNPCIntelKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Intel", "npc_intel_refresh")
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

// renderNPCIntelReport runs a fresh tactical read and returns the
// formatted text plus its attached keyboard, shared by the
// /npc_intel command and its refresh callback so the two can never
// drift apart.
func (h *NPCIntelHandler) renderNPCIntelReport(ctx context.Context, userID int64) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Intel.Recommend(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	return npcintel.FormatForTelegram(rec), buildNPCIntelKeyboard(), nil
}

// ── /npc_intel ───────────────────────────────────────────────────────
//
// Gives a tactical, composition-specific read of the Rogue Drone Nest
// (scaled to the player's level) against the player's own current
// mobile fleet. Read-only: never launches a raid or moves any unit.
func (h *NPCIntelHandler) HandleNPCIntel(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	ctx := context.Background()
	text, keyboard, err := h.renderNPCIntelReport(ctx, sender.ID)
	if errors.Is(err, npcintel.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI NPC Intelligence advisor is temporarily unavailable: " + err.Error())
	}

	return c.Send(text, keyboard)
}

// ── callback: npc_intel_refresh ──────────────────────────────────────
//
// Re-runs the tactical read on demand (a real new AI Foundation call,
// subject to the usual cost/cache/budget rules) and posts a fresh
// report.
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
	return c.Send(text, keyboard)
}
