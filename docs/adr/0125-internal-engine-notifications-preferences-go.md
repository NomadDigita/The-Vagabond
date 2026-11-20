# ADR 0125: Decisions from `internal/engine/notifications/preferences.go`

Status: Accepted

These decisions were written as comments in `internal/engine/notifications/preferences.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MutableCategories is the single source of truth for which notification

Removed from `internal/engine/notifications/preferences.go:8`.

~~~~text
MutableCategories is the single source of truth for which notification
categories a player is allowed to mute. Nothing outside this set can ever
be silenced by notification_preferences - see
MMO_WORLD_EVOLUTION_PLAN.md Phase 7 milestone 2: "without suppressing
combat, discovery, or supply-loss alerts." Adding a new mutable category
means deliberately deciding it's as low-stakes as route_status (a
peaceful road pass, a cleared weather incident, a convoy arrival) - not
just tagging more call sites with an existing one.
~~~~


## scout_status: 2026-08-06 direct request - long-range scouting's

Removed from `internal/engine/notifications/preferences.go:18`.

~~~~text
scout_status: 2026-08-06 direct request - long-range scouting's
periodic pings ("still searching", "en route home", "party
returned") used to share the single route_status toggle with
unrelated road/convoy chatter. Split out so a player can silence
scouting noise without also silencing road-encounter/convoy
status. "CONTACT!"/"SPOTTED" discovery events are never tagged
with this - see scoutMissionFindsTarget's doc comment.
~~~~


## querier is satisfied by both *sql.DB and *sql.Tx, so tick-engine code

Removed from `internal/engine/notifications/preferences.go:28`.

~~~~text
querier is satisfied by both *sql.DB and *sql.Tx, so tick-engine code
(which always has an open transaction) and bot handlers (which usually
don't) can share this same queuing helper.
~~~~


## IsCategoryMuted reports whether userID has muted category. Categories

Removed from `internal/engine/notifications/preferences.go:36`.

~~~~text
IsCategoryMuted reports whether userID has muted category. Categories
outside MutableCategories are never muted, regardless of what's in the
database - this is checked in Go, not left to a DB default, so a bad row
or a future column added without updating MutableCategories can't
accidentally suppress a combat/discovery/supply-loss alert.
~~~~


## no row yet == default (unmuted)

Removed from `internal/engine/notifications/preferences.go:50`.

~~~~text
no row yet == default (unmuted)
~~~~


## no row yet == default (unmuted) (2)

Removed from `internal/engine/notifications/preferences.go:55`.

~~~~text
no row yet == default (unmuted)
~~~~


## Queue inserts a notification tagged with category, unless the recipient

Removed from `internal/engine/notifications/preferences.go:61`.

~~~~text
Queue inserts a notification tagged with category, unless the recipient
has muted that category. Combat/discovery/supply-loss call sites should
keep using category "general" (or call Queue with a category that isn't
in MutableCategories) so they're never subject to muting at all.

effectID is optional and variadic purely so every one of this
codebase's ~50 existing call sites keeps compiling unchanged - pass
one of the Effect* constants (notifications.go) to have
Dispatcher.drainQueue deliver this notification with a Telegram
message-effect animation attached; pass nothing (or "") for the
pre-2026-08-06 behavior. Only the first variadic value is used.
~~~~
