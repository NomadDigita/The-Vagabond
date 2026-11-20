# ADR 0180: Decisions from `internal/game/fleetcommander/commander.go`

Status: Accepted

These decisions were written as comments in `internal/game/fleetcommander/commander.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/fleetcommander/commander.go:13`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from governor.MemoryScope and any future scope.
~~~~


## unitColumns lists every workshop_inventory unit column this package

Removed from `internal/game/fleetcommander/commander.go:31`.

~~~~text
unitColumns lists every workshop_inventory unit column this package
knows about. Kept as an explicit list (rather than SELECT *) so a
schema change is a deliberate one-line addition here, not a silent
behavior change.

Verified complete against origin/main as of 2026-07-16 by
cross-referencing every CREATE TABLE column and every
`ALTER TABLE workshop_inventory ADD COLUMN` statement in
cmd/bot/main.go (28 real unit/garrison columns, excluding the
encampment_id foreign key) — not assumed. This list was previously
missing 10 columns added by the parallel SpaceHunt Phase 6/7 combat
work (liberators, wraiths, observers, guardians, piercing_missiles,
cargo_mk1/2/3, garrisoned_soldiers, garrisoned_mechs), which would
have made Fleet Commander silently blind to a meaningful slice of a
player's real fleet strength — not a crash (those columns are all
properly ALTER-guarded), but a real analysis-quality gap. Re-run the
cross-reference in PROJECT_MASTER_PLAN.md's audit notes before
trusting this list is still complete after any future SpaceHunt
combat-phase merge.
~~~~


## No workshop_inventory row yet (brand new player) — an empty

Removed from `internal/game/fleetcommander/commander.go:77`.

~~~~text
No workshop_inventory row yet (brand new player) — an empty
fleet is a valid, meaningful state, not an error.
~~~~


## BuildRogueNestTarget produces a PvE TargetProfile scaled to the

Removed from `internal/game/fleetcommander/commander.go:103`.

~~~~text
BuildRogueNestTarget produces a PvE TargetProfile scaled to the
player's level, reusing the exact same composition logic already
shown to players via /recon (internal/game/content.RogueNestComposition)
so the Fleet Commander's view of the nest never disagrees with what
the static recon report already told the player.

As of the parallel SpaceHunt Phase 7 work ("Rogue AI scaling via
real player subsystems"), RogueNestForce carries a full per-turret
Defense Grid, Guardian/Observer garrison, research level, shields,
and an optional hero-equivalent superpower — all surfaced here, not
just the legacy flat TurretBonus.
~~~~


## Recommend produces a fresh AI recommendation for a player's fleet

Removed from `internal/game/fleetcommander/commander.go:186`.

~~~~text
Recommend produces a fresh AI recommendation for a player's fleet
against the (currently PvE-only, see PROJECT_MASTER_PLAN.md) rogue
nest target. It stores both turns in ai_memory under MemoryScope.

This is advisory-only: nothing in this method launches a raid or
moves any unit.
~~~~
