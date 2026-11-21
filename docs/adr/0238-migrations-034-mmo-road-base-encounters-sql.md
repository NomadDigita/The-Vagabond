# ADR 0238: Decisions from `migrations/034_mmo_road_base_encounters.sql`

Status: Accepted

These decisions were written as comments in `migrations/034_mmo_road_base_encounters.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## ==============================================================================

Removed from `migrations/034_mmo_road_base_encounters.sql:1`.

~~~~text
==============================================================================
THE VAGABOND - MMO LIVING WORLD: ROAD-VS-BASE ENCOUNTERS
(034_mmo_road_base_encounters.sql)
DB Engine: PostgreSQL (Supabase)

Completes MMO_WORLD_EVOLUTION_PLAN.md Phase 4 milestone 2, which shipped
expedition-vs-expedition road encounters (030) but not the "expeditions
and bases" half: a moving column passing near a passive player base only
ever produced a Phase 2 discovery, never a forced Attack/Continue window.

Deliberately a separate table from road_encounters rather than widening
it to allow a NULL raid_b_id: road_encounters' ordering/uniqueness
constraints (raid_a_id < raid_b_id, both NOT NULL) assume two raids, and
the already-verified expedition-vs-expedition path should not be
disturbed to accommodate a one-sided (raid vs. stationary base) case.
==============================================================================
~~~~


## Only one live decision window per (expedition, base) pair at a time.

Removed from `migrations/034_mmo_road_base_encounters.sql:32`.

~~~~text
Only one live decision window per (expedition, base) pair at a time.
~~~~


## Same guarded-FK pattern as active_encounter_id/road_encounters in 030,

Removed from `migrations/034_mmo_road_base_encounters.sql:42`.

~~~~text
Same guarded-FK pattern as active_encounter_id/road_encounters in 030,
since road_base_encounters is defined above this ALTER in the same file.
~~~~
