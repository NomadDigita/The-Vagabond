# ADR 0140: Decisions from `internal/engine/tick/aispawning.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aispawning.go` and moved here so the source no longer carries them. Each entry is the original comment.

## aiFactionSpawnInterval is how often exactly one new AI faction is

Removed from `internal/engine/tick/aispawning.go:11`.

~~~~text
aiFactionSpawnInterval is how often exactly one new AI faction is
spawned into the world. Set to 24h/3 so three new factions appear per
day, per the project owner's direct answer 2026-08-01 to
AI_AND_SCOUTING_EXPANSION_PLAN.md item 1's open question ("3/day").
Deliberately a fixed interval rather than a daily-quota-with-reset
scheme - that avoids any day-boundary ambiguity (which timezone counts
as "today," whether a quota resets mid-tick) while producing the same
long-run rate, and mirrors how aiDecisionCadence already handles
per-faction pacing with a plain interval rather than a calendar quota.
~~~~


## aiFactionSpawnStartingLevel is deliberately the weakest tier in the

Removed from `internal/engine/tick/aispawning.go:22`.

~~~~text
aiFactionSpawnStartingLevel is deliberately the weakest tier in the
game - even lower than the original 8 seeded factions' weaker starting
level of 3 (see seedAICivilizations in cmd/bot/main.go) - so a
continuously growing AI population doesn't front-load the world with
instantly-dangerous new threats. growAICivilizations' existing passive
leveling ramps a spawn up over time exactly like it already does for
the original 8; there's nothing spawn-specific about how a faction
grows once it exists, and nothing spawn-specific about how
decideAIFactionActions scouts or raids with it either.
~~~~


## aiFactionSpawnRegions mirrors the four-quadrant continent scheme

Removed from `internal/engine/tick/aispawning.go:33`.

~~~~text
aiFactionSpawnRegions mirrors the four-quadrant continent scheme
seedAICivilizations uses (coordinates.region), cycled round-robin by
the spawn counter below so growth stays roughly balanced across
continents rather than clustering in whichever one gets rolled most.
~~~~


## aiFactionSpawnPrefixes and aiFactionSpawnSuffixes generate a flavor

Removed from `internal/engine/tick/aispawning.go:39`.

~~~~text
aiFactionSpawnPrefixes and aiFactionSpawnSuffixes generate a flavor
name in the same two-word "Adjective Faction-type" shape as the
original 8 hand-authored names (e.g. "Ironclad Directive", "Sandrunner
Clan") via a simple combiner rather than a much larger authored list -
ai_faction_key, not the display name, is what guarantees uniqueness,
so two spawned factions sharing a display name is harmless flavor
overlap, not a bug.
~~~~


## spawnNewAIFactions is Phase 6's continuous-growth counterpart to

Removed from `internal/engine/tick/aispawning.go:60`.

~~~~text
spawnNewAIFactions is Phase 6's continuous-growth counterpart to
seedAICivilizations' one-time boot seed - see
AI_AND_SCOUTING_EXPANSION_PLAN.md item 1 for the full design record,
including the project owner's exact answer this implements. A brand
new AI faction is created as nothing but another is_ai_faction = TRUE
encampments row with matching users/resources/workshop_inventory rows
- the same shape seedAICivilizations produces - so it needs zero
special-casing anywhere else: growAICivilizations grows it,
decideAIFactionActions scouts/raids with it, and a human's own
exploration can discover it exactly like the original 8.

world_state.id = 1's last_ai_spawn_at is the world-scoped (not
per-faction) cadence gate; ai_factions_spawned_count both records how
many have spawned and supplies each new faction's unique
ai_faction_key/telegram_id, so it's read/written under the same
FOR UPDATE lock as the cadence check to stay race-free across
concurrent tick workers, mirroring how decideAIFactionActions guards
against a double-decision with FOR UPDATE OF e.
~~~~


## Comfortably clear of the hand-seeded -900001..-900008 range so the

Removed from `internal/engine/tick/aispawning.go:91`.

~~~~text
Comfortably clear of the hand-seeded -900001..-900008 range so the
two schemes can never collide.
~~~~


## World-visible but low-key flavor, same spirit as an AI raid's

Removed from `internal/engine/tick/aispawning.go:165`.

~~~~text
World-visible but low-key flavor, same spirit as an AI raid's
world_news headline in launchAIRaid - a new outpost appearing is
public information, not a targeted alert, so it doesn't go through
the notifications.QueueToRegion path weather events use.
~~~~
