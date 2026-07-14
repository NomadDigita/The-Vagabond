package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"gopkg.in/telebot.v3"
)

// AIStatusHandler exposes the Phase A AI Foundation to players and
// admins. It intentionally uses new, previously-unused command names
// (/ai_status, /ai_settings) so it cannot collide with any command
// registered by the parallel SpaceHunt phases 1-6 roadmap.
type AIStatusHandler struct {
	Service  *ai.Service
	AdminIDs []int64
}

func NewAIStatusHandler(service *ai.Service, adminIDStrs string) *AIStatusHandler {
	var ids []int64
	for _, s := range strings.Split(adminIDStrs, ",") {
		trimmed := strings.TrimSpace(s)
		if val, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			ids = append(ids, val)
		}
	}
	return &AIStatusHandler{Service: service, AdminIDs: ids}
}

func (h *AIStatusHandler) isAdmin(id int64) bool {
	for _, a := range h.AdminIDs {
		if a == id {
			return true
		}
	}
	return false
}

// ── /ai_status (admin only) ────────────────────────────────────────
//
// Shows provider availability, global feature flags, and today's
// global AI spend against the configured budget. Read-only.
func (h *AIStatusHandler) HandleAIStatus(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.isAdmin(sender.ID) {
		return c.Send("⛔ Administrator access required.")
	}

	ctx := context.Background()
	var b strings.Builder
	b.WriteString("🤖 AI FOUNDATION STATUS (Phase A)\n\n")

	b.WriteString("Providers (fallback order):\n")
	for _, p := range h.Service.Registry.Ordered() {
		b.WriteString(fmt.Sprintf("  • %s — available ✅\n", p.Name()))
	}
	if len(h.Service.Registry.Ordered()) == 0 {
		b.WriteString("  (none available — this should never happen; mock provider should always be registered)\n")
	}

	if h.Service.Permissions != nil {
		flags, err := h.Service.Permissions.GlobalFlags(ctx)
		if err == nil {
			b.WriteString("\nGlobal feature flags:\n")
			for _, f := range ai.AllFeatures() {
				state := "✅ enabled"
				if !flags[f] {
					state = "🚫 disabled"
				}
				b.WriteString(fmt.Sprintf("  • %-24s %s\n", f, state))
			}
		}
	}

	if h.Service.Cost != nil {
		global, err := h.Service.Cost.GlobalSpendToday(ctx)
		if err == nil {
			b.WriteString(fmt.Sprintf("\nGlobal spend today: $%.4f / $%.2f budget\n", global, h.Service.Config.MaxGlobalCostPerDayUSD))
		}
	}

	b.WriteString(fmt.Sprintf("\nMaster switch: %v | Per-user daily cap: $%.2f | Cache TTL: %ds",
		h.Service.Config.Enabled, h.Service.Config.MaxUserCostPerDayUSD, h.Service.Config.CacheTTLSeconds))

	return c.Send(b.String())
}

// ── /ai_status_toggle (admin only, callback-free simple command) ───
//
// Usage: /ai_status_toggle <feature> <on|off>
// Flips a global feature flag without requiring a redeploy.
func (h *AIStatusHandler) HandleAIStatusToggle(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.isAdmin(sender.ID) {
		return c.Send("⛔ Administrator access required.")
	}

	parts := strings.Fields(c.Message().Payload)
	if len(parts) != 2 {
		return c.Send("Usage: /ai_status_toggle <feature> <on|off>\nSee /ai_status for valid feature names.")
	}
	feature := ai.Feature(parts[0])
	enabled := strings.EqualFold(parts[1], "on")

	if h.Service.Permissions == nil {
		return c.Send("⚠️ Permission subsystem unavailable.")
	}
	if err := h.Service.Permissions.SetGlobalFlag(context.Background(), feature, enabled); err != nil {
		return c.Send(fmt.Sprintf("⚠️ Failed to update flag: %v", err))
	}
	return c.Send(fmt.Sprintf("✅ %s is now %s globally.", feature, map[bool]string{true: "ENABLED", false: "DISABLED"}[enabled]))
}

// ── /ai_settings (any player) ───────────────────────────────────────
//
// Usage:
//
//	/ai_settings                       — list your current preferences
//	/ai_settings <feature> <on|off>    — opt yourself in/out of a feature
func (h *AIStatusHandler) HandleAISettings(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	payload := strings.TrimSpace(c.Message().Payload)
	if payload == "" {
		var b strings.Builder
		b.WriteString("🤖 YOUR AI PREFERENCES\n\n")
		b.WriteString("All AI-assisted features are ON by default. Use:\n")
		b.WriteString("/ai_settings <feature> <on|off>\n\nFeatures:\n")
		for _, f := range ai.AllFeatures() {
			b.WriteString(fmt.Sprintf("  • %s\n", f))
		}
		return c.Send(b.String())
	}

	parts := strings.Fields(payload)
	if len(parts) != 2 {
		return c.Send("Usage: /ai_settings <feature> <on|off>")
	}
	feature := ai.Feature(parts[0])
	valid := false
	for _, f := range ai.AllFeatures() {
		if f == feature {
			valid = true
			break
		}
	}
	if !valid {
		return c.Send("❌ Unknown feature name. Run /ai_settings with no arguments to see the list.")
	}
	enabled := strings.EqualFold(parts[1], "on")

	if h.Service.Permissions == nil {
		return c.Send("⚠️ Permission subsystem unavailable.")
	}
	if err := h.Service.Permissions.SetUserPreference(ctx, sender.ID, feature, enabled); err != nil {
		return c.Send(fmt.Sprintf("⚠️ Failed to save preference: %v", err))
	}
	return c.Send(fmt.Sprintf("✅ %s is now %s for you.", feature, map[bool]string{true: "ON", false: "OFF"}[enabled]))
}
