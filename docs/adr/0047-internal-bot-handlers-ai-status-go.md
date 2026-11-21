# ADR 0047: Decisions from `internal/bot/handlers/ai_status.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/ai_status.go` and moved here so the source no longer carries them. Each entry is the original comment.

## AIStatusHandler exposes the Phase A AI Foundation to players and

Removed from `internal/bot/handlers/ai_status.go:16`.

~~~~text
AIStatusHandler exposes the Phase A AI Foundation to players and
admins. It intentionally uses new, previously-unused command names
(/ai_status, /ai_settings) so it cannot collide with any command
registered by the parallel SpaceHunt phases 1-6 roadmap.
~~~~


## ── /ai_status (admin only) ────────────────────────────────────────

Removed from `internal/bot/handlers/ai_status.go:45`.

~~~~text
── /ai_status (admin only) ────────────────────────────────────────

Shows provider availability, global feature flags, and today's
global AI spend against the configured budget. Read-only.
~~~~


## The list above only shows whether a key is *configured*, not

Removed from `internal/bot/handlers/ai_status.go:94`.

~~~~text
The list above only shows whether a key is *configured*, not
whether the provider actually works — a provider with a bad
key, an exhausted quota, or an unpurchased model still shows up
here as "available ✅" (Available() only checks for a non-empty
key) and silently falls through to mock at request time. Point
straight at the live diagnostic rather than let another session
re-derive that gap from scratch.
~~~~


## ── /ai_probe (admin only) ──────────────────────────────────────────

Removed from `internal/bot/handlers/ai_status.go:106`.

~~~~text
── /ai_probe (admin only) ──────────────────────────────────────────

Live-tests every registered, non-mock provider with one minimal
real completion call each and reports the exact success/failure
result, including the provider's own error text on failure. Unlike
/ai_status's "Providers" list (which only ever shows whether a key
is *configured*, not whether it actually works), this answers "is
this provider genuinely reachable and usable right now" directly
from Telegram — no Render dashboard/log access required. Costs
count against the normal daily budget; each call is capped at 16
output tokens, so the cost per run is negligible relative to a real
advisor request.
~~~~


## ── /ai_status_toggle (admin only, callback-free simple command) ───

Removed from `internal/bot/handlers/ai_status.go:149`.

~~~~text
── /ai_status_toggle (admin only, callback-free simple command) ───

Usage: /ai_status_toggle <feature> <on|off>
Flips a global feature flag without requiring a redeploy.
~~~~


## ── /ai_settings (any player) ───────────────────────────────────────

Removed from `internal/bot/handlers/ai_status.go:178`.

~~~~text
── /ai_settings (any player) ───────────────────────────────────────

Usage:

	/ai_settings                       — list your current preferences
	/ai_settings <feature> <on|off>    — opt yourself in/out of a feature
~~~~
