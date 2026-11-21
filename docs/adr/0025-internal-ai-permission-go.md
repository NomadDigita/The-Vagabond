# ADR 0025: Decisions from `internal/ai/permission.go`

Status: Accepted

These decisions were written as comments in `internal/ai/permission.go` and moved here so the source no longer carries them. Each entry is the original comment.

## PermissionManager decides whether a given (userID, feature) request

Removed from `internal/ai/permission.go:8`.

~~~~text
PermissionManager decides whether a given (userID, feature) request
is allowed to run. Two layers of control exist, checked in order:

 1. Global feature flags (ai_feature_flags table) — an admin kill
    switch for an entire AI subsystem, independent of any one
    player. Defaults to enabled if no row exists, so shipping a new
    Feature never requires a migration to turn it on.
 2. Per-user opt-in/opt-out (ai_permissions table) — lets a player
    disable AI assistance for themselves (e.g. "I want to fly the
    Fleet Commander manually"), or lets staff disable AI for one
    abusive account without flipping the global switch.

The player's own explicit choice is always respected; automation
beyond "recommend" (e.g. autopilot mode in the Planet Governor) is a
separate, stricter opt-in enforced by the Phase B handler itself,
not by this package.
~~~~


## IsAllowed returns whether the request may proceed, and if not, a

Removed from `internal/ai/permission.go:32`.

~~~~text
IsAllowed returns whether the request may proceed, and if not, a
short human-readable reason suitable for showing the player.
~~~~


## Background/system request — no per-user row applies.

Removed from `internal/ai/permission.go:47`.

~~~~text
Background/system request — no per-user row applies.
~~~~


## GlobalFlags returns the current enabled/disabled state of every

Removed from `internal/ai/permission.go:87`.

~~~~text
GlobalFlags returns the current enabled/disabled state of every
known feature, defaulting missing rows to enabled=true.
~~~~
