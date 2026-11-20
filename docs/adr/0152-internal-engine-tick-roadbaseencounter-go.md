# ADR 0152: Decisions from `internal/engine/tick/roadbaseencounter.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/roadbaseencounter.go` and moved here so the source no longer carries them. Each entry is the original comment.

## evaluateRoadBaseEncounters completes MMO_WORLD_EVOLUTION_PLAN.md Phase 4

Removed from `internal/engine/tick/roadbaseencounter.go:14`.

~~~~text
evaluateRoadBaseEncounters completes MMO_WORLD_EVOLUTION_PLAN.md Phase 4
milestone 2 ("expeditions and bases"): discoverRouteContacts already
reveals a passive base to a passing expedition, but never forced a
decision - a column would just keep marching. This mirrors
evaluateRoadEncounters (expedition-vs-expedition) but against a
stationary base instead of a second moving column.

AI factions were previously excluded here too (encampments.is_ai_faction
= FALSE), matching the same exclusion pattern AI-vs-AI raids and
scouting used to have - removed per
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.4: "everything
should affect them too." An AI faction has never set a manual home-
defense garrison (workshop_inventory.garrisoned_soldiers/
garrisoned_mechs - nobody's played its Hero Commander panel), so
HandleRoadBaseEncounterCallback in combat_road_encounters.go gives it a
synthetic reserve computed from its live soldiers/mechs pool instead of
reading that always-zero column - see loadBaseGarrisonForce's doc
comment for the full reasoning on why a straight parity fix would have
been a free, uncontested win for any human passing by.

Runs its own fresh query rather than reusing evaluateRoadEncounters'
in-memory movers slice, since that slice may already be stale within the
same tick (a raid resolved into an expedition-vs-expedition encounter
there is no longer eligible here).
~~~~


## Cooldown: same shape as evaluateRoadEncounters, so a

Removed from `internal/engine/tick/roadbaseencounter.go:107`.

~~~~text
Cooldown: same shape as evaluateRoadEncounters, so a
"Continue" past this base isn't immediately re-asked.
~~~~


## another tick worker already created this pending encounter

Removed from `internal/engine/tick/roadbaseencounter.go:132`.

~~~~text
another tick worker already created this pending encounter
~~~~


## Freeze the column exactly like an expedition-vs-expedition

Removed from `internal/engine/tick/roadbaseencounter.go:137`.

~~~~text
Freeze the column exactly like an expedition-vs-expedition
encounter - no arrival/return processing until resolved.
~~~~


## Discovery is reciprocal even if nobody chooses to fight,

Removed from `internal/engine/tick/roadbaseencounter.go:143`.

~~~~text
Discovery is reciprocal even if nobody chooses to fight,
matching discoverRouteContacts' existing behavior.
~~~~


## expireRoadBaseEncounters resolves any pending base encounter whose

Removed from `internal/engine/tick/roadbaseencounter.go:163`.

~~~~text
expireRoadBaseEncounters resolves any pending base encounter whose
response window has lapsed as a peaceful pass, and unfreezes the column
from exactly the position it paused at - same shape as
expireRoadEncounters/resolveEncounterAsContinue for expedition-vs-expedition.
~~~~
