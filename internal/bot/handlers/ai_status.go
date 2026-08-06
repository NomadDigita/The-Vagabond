package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
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
		return c.Send("⛔ Administrator access required.", keyboards.AdvisorsNavigation())
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

	b.WriteString(fmt.Sprintf("\nMaster switch: %v | Per-user daily cap: $%.2f | Cache TTL: %s",
		h.Service.Config.Enabled, h.Service.Config.MaxUserCostPerDayUSD, formatDuration(time.Duration(h.Service.Config.CacheTTLSeconds)*time.Second)))

	// The list above only shows whether a key is *configured*, not
	// whether the provider actually works — a provider with a bad
	// key, an exhausted quota, or an unpurchased model still shows up
	// here as "available ✅" (Available() only checks for a non-empty
	// key) and silently falls through to mock at request time. Point
	// straight at the live diagnostic rather than let another session
	// re-derive that gap from scratch.
	b.WriteString("\n\nRun /ai_probe for a live test call to each provider above — it will show the exact error (quota, auth, model access, etc.) if one is silently falling back to mock.")

	return c.Send(b.String(), keyboards.AdvisorsNavigation())
}

// ── /ai_probe (admin only) ──────────────────────────────────────────
//
// Live-tests every registered, non-mock provider with one minimal
// real completion call each and reports the exact success/failure
// result, including the provider's own error text on failure. Unlike
// /ai_status's "Providers" list (which only ever shows whether a key
// is *configured*, not whether it actually works), this answers "is
// this provider genuinely reachable and usable right now" directly
// from Telegram — no Render dashboard/log access required. Costs
// count against the normal daily budget; each call is capped at 16
// output tokens, so the cost per run is negligible relative to a real
// advisor request.
func (h *AIStatusHandler) HandleAIProbe(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.isAdmin(sender.ID) {
		return c.Send("⛔ Administrator access required.", keyboards.AdvisorsNavigation())
	}

	_ = c.Send("🔬 Probing every configured AI provider with a live test call — this takes a few seconds…", keyboards.AdvisorsNavigation())

	ctx := context.Background()
	results := h.Service.ProbeAllProviders(ctx)

	var b strings.Builder
	b.WriteString("🔬 AI PROVIDER PROBE RESULTS\n\n")
	if len(results) == 0 {
		b.WriteString("No non-mock providers are registered.\n")
	}
	for _, r := range results {
		if r.OK {
			b.WriteString(fmt.Sprintf("✅ %s — OK (model: %s, %s)\n", r.Provider, r.Model, r.Latency.Round(10*time.Millisecond)))
			continue
		}
		b.WriteString(fmt.Sprintf("❌ %s — FAILED\n     ↳ %s\n", r.Provider, r.Err))
	}
	b.WriteString("\nA ❌ here (not \"not configured\") means the key is set but the provider itself is rejecting the request — quota exhausted, model not activated on the account, bad key, etc. That exact reason is above; this is the same text a fix would otherwise require Render log access to see.")

	return c.Send(b.String(), keyboards.AdvisorsNavigation())
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
		return c.Send("⛔ Administrator access required.", keyboards.AdvisorsNavigation())
	}

	parts := strings.Fields(c.Message().Payload)
	if len(parts) != 2 {
		return c.Send("Usage: /ai_status_toggle <feature> <on|off>\nSee /ai_status for valid feature names.", keyboards.AdvisorsNavigation())
	}
	feature := ai.Feature(parts[0])
	enabled := strings.EqualFold(parts[1], "on")

	if h.Service.Permissions == nil {
		return c.Send("⚠️ Permission subsystem unavailable.", keyboards.AdvisorsNavigation())
	}
	if err := h.Service.Permissions.SetGlobalFlag(context.Background(), feature, enabled); err != nil {
		return c.Send(fmt.Sprintf("⚠️ Failed to update flag: %v", err), keyboards.AdvisorsNavigation())
	}
	return c.Send(fmt.Sprintf("✅ %s is now %s globally.", feature, map[bool]string{true: "ENABLED", false: "DISABLED"}[enabled]), keyboards.AdvisorsNavigation())
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
		return c.Send(b.String(), keyboards.AdvisorsNavigation())
	}

	parts := strings.Fields(payload)
	if len(parts) != 2 {
		return c.Send("Usage: /ai_settings <feature> <on|off>", keyboards.AdvisorsNavigation())
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
		return c.Send("❌ Unknown feature name. Run /ai_settings with no arguments to see the list.", keyboards.AdvisorsNavigation())
	}
	enabled := strings.EqualFold(parts[1], "on")

	if h.Service.Permissions == nil {
		return c.Send("⚠️ Permission subsystem unavailable.", keyboards.AdvisorsNavigation())
	}
	if err := h.Service.Permissions.SetUserPreference(ctx, sender.ID, feature, enabled); err != nil {
		return c.Send(fmt.Sprintf("⚠️ Failed to save preference: %v", err), keyboards.AdvisorsNavigation())
	}
	return c.Send(fmt.Sprintf("✅ %s is now %s for you.", feature, map[bool]string{true: "ON", false: "OFF"}[enabled]), keyboards.AdvisorsNavigation())
}
