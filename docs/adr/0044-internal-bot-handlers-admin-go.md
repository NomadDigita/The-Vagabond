# ADR 0044: Decisions from `internal/bot/handlers/admin.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/admin.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Phase 7 (item 13): admin panel consolidation. Tracks which admin

Removed from `internal/bot/handlers/admin.go:28`.

~~~~text
Phase 7 (item 13): admin panel consolidation. Tracks which admin
(by Telegram ID) is mid-flow on a button that needs a free-text
argument (e.g. "Gift Premium" needs a username + day count), so
their next plain-text message can be consumed as that argument
instead of falling through to normal NLP parsing. See
HandleAdminPendingInput, wired ahead of nlp.HandleTextMessage in
main.go's OnText registration.
~~~~


## Phase 7 (item 13): admin panel consolidation. Every admin action

Removed from `internal/bot/handlers/admin.go:75`.

~~~~text
Phase 7 (item 13): admin panel consolidation. Every admin action
used to be split across this panel (4 of them), 9 separate
/admin_* slash commands, and 4 duplicate bottom-menu buttons -
several were exact copy-pasted logic reachable 3 different ways
(see SPACEHUNT_PHASE7_LOG.md). This panel is now the single
discoverable place for all of them; the old slash commands still
work unchanged for muscle memory, but every one of them now
delegates to the same shared do*/db* helper this panel calls.
~~~~


## Previously this sent panelText directly via c.Send with BOTH the

Removed from `internal/bot/handlers/admin.go:107`.

~~~~text
Previously this sent panelText directly via c.Send with BOTH the
inline selector and keyboards.AdminNavigation() passed as separate
ReplyMarkup arguments in one call - telebot only honors one
*telebot.ReplyMarkup per message, so the inline one was silently
dropped. Every admin action button (Tick, Inject, Gift Premium,
Broadcast, Publish Changelog, all of them) never actually
rendered. This is the exact "single-ReplyMarkup collision" this
codebase's own sendPanelWithNav/sendPanelWithNavHTML helpers exist
to prevent (see navhelper.go) - use it here too, like every other
panel already does. See send_markup_guard_test.go for the
regression guard against this reappearing anywhere else.
~~~~


## adminPromptFor gives the guided-input prompt text for each action that

Removed from `internal/bot/handlers/admin.go:121`.

~~~~text
adminPromptFor gives the guided-input prompt text for each action that
needs a free-text argument after the button tap, plus registers the
pending state so HandleAdminPendingInput knows how to parse the
admin's next message.
~~~~


## Tick-engine + world-event liveness. Added 2026-08-02 after a

Removed from `internal/bot/handlers/admin.go:189`.

~~~~text
Tick-engine + world-event liveness. Added 2026-08-02 after a
player-reported "world events never fire" investigation that
had no way to distinguish "the tick engine isn't running in
this deployment" from "it's running, just rolling misses" -
both looked identical from inside the game. See
internal/engine/tick/engine.go's LastTickStatus and
internal/engine/world/weather.go's per-tick heartbeat log for
the underlying instrumentation this reads/complements.
~~~~


## Destructive - require a second, explicit confirmation tap

Removed from `internal/bot/handlers/admin.go:239`.

~~~~text
Destructive - require a second, explicit confirmation tap
rather than executing immediately. This was a real safety
gap in the panel path before item 13 (the /admin_db_reset
slash command had the exact same gap, and still does - typing
it out is itself a kind of confirmation, so that's left as-is).
~~~~


## HandleAdminPendingInput consumes an admin's next free-text message if

Removed from `internal/bot/handlers/admin.go:262`.

~~~~text
HandleAdminPendingInput consumes an admin's next free-text message if
(and only if) they're mid-flow on a guided-input action from the
consolidated /admin panel above. Returns handled=false immediately
for anyone with no pending action, so normal NLP text parsing
(nlp.HandleTextMessage) continues completely unaffected - see
main.go's OnText registration for how the two are chained.
~~~~


## doDBReset is the single source of truth for the destructive database

Removed from `internal/bot/handlers/admin.go:373`.

~~~~text
doDBReset is the single source of truth for the destructive database
reset, shared by /admin_db_reset and the consolidated /admin panel's
two-tap confirm flow (Phase 7 item 13) - the panel path didn't have
any confirmation step before this, which was a real safety gap for
something this destructive.
~~~~


## Safe Refunding Loop: Refund all active campaigns (and staged co-op lobbies) before erasing

Removed from `internal/bot/handlers/admin.go:385`.

~~~~text
Safe Refunding Loop: Refund all active campaigns (and staged co-op lobbies) before erasing states
~~~~


## doGiftPremium is the single source of truth for granting Premium,

Removed from `internal/bot/handlers/admin.go:527`.

~~~~text
doGiftPremium is the single source of truth for granting Premium,
shared by /admin_gift_premium and the consolidated /admin panel's
guided-input flow (Phase 7 item 13).
~~~~


## doGiftResources is the single source of truth for gifting a resource

Removed from `internal/bot/handlers/admin.go:580`.

~~~~text
doGiftResources is the single source of truth for gifting a resource
to a player, shared by /admin_gift_resources and the consolidated
/admin panel's guided-input flow (Phase 7 item 13).
~~~~


## doInjectSelf is the single source of truth for injecting 5,000 of

Removed from `internal/bot/handlers/admin.go:633`.

~~~~text
doInjectSelf is the single source of truth for injecting 5,000 of
every resource into the admin's own camp - shared by /admin_give, the
"🪙 Inject Resources" bottom-menu button, and the consolidated /admin
panel's "inject" callback (Phase 7 item 13). All three were
independently maintaining the same UPDATE statement before this, and
had quietly drifted - this version (with neuro_cores included) is now
the canonical one.
~~~~


## doSetTaxRate is the single source of truth for the daily tax rate,

Removed from `internal/bot/handlers/admin.go:686`.

~~~~text
doSetTaxRate is the single source of truth for the daily tax rate,
shared by /settaxrate and the consolidated /admin panel's
guided-input flow (Phase 7 item 13).
~~~~


## doFactionChange is the single source of truth for an admin's own

Removed from `internal/bot/handlers/admin.go:732`.

~~~~text
doFactionChange is the single source of truth for an admin's own
self-directed faction swap, shared by /admin_faction and the
consolidated /admin panel's guided-input flow (Phase 7 item 13).
~~~~


## doBroadcast is the single source of truth for a system-wide broadcast,

Removed from `internal/bot/handlers/admin.go:773`.

~~~~text
doBroadcast is the single source of truth for a system-wide broadcast,
shared by /admin_broadcast and the consolidated /admin panel's
guided-input flow (Phase 7 item 13).
~~~~
