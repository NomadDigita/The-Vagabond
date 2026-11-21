# ADR 0050: Decisions from `internal/bot/handlers/boss.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/boss.go` and moved here so the source no longer carries them. Each entry is the original comment.

## tier2KillRequirement/tier3KillRequirement gate a higher tier's

Removed from `internal/bot/handlers/boss.go:44`.

~~~~text
tier2KillRequirement/tier3KillRequirement gate a higher tier's
Attack button until enough lower-tier kills have happened server-
wide - see schema.go's own Phase 8 comment for why "tiering" means
gating an always-live boss's attackability rather than gating a
spawn roll (world_bosses has no spawn mechanic; it's a small, fixed
set that respawns to full HP after each kill).
~~~~


## tierUnlocked reports whether tier is currently attackable: tier 1 is

Removed from `internal/bot/handlers/boss.go:55`.

~~~~text
tierUnlocked reports whether tier is currently attackable: tier 1 is
always unlocked; tier 2 requires tier2KillRequirement combined kills
across every tier-1 boss; tier 3 requires tier3KillRequirement
combined kills across every tier-2 boss (which includes The
Ravager, since Gauntlet bosses still count as their own tier for
this gate).
~~~~


## Committed troops are actually at risk now - deduct them from the

Removed from `internal/bot/handlers/boss.go:185`.

~~~~text
Committed troops are actually at risk now - deduct them from the
garrison immediately, exactly like drafting into a real raid. The
boss retaliates on arrival, and only survivors march home.
~~~~
