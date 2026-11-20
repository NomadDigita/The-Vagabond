# ADR 0234: Decisions from `migrations/030_mmo_route_legs_and_road_encounters.sql`

Status: Accepted

These decisions were written as comments in `migrations/030_mmo_route_legs_and_road_encounters.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Each raid's "leg" is the current uninterrupted movement segment (the

Removed from `migrations/030_mmo_route_legs_and_road_encounters.sql:11`.

~~~~text
Each raid's "leg" is the current uninterrupted movement segment (the
outbound march or the return march). leg_started_at/leg_total_minutes let
the engine compute a stable 0..1 route-progress fraction for the CURRENT
leg without being confused by round-trip resolve_time reuse across the
marching -> engaged -> returning state machine. movement_state gates
whether that fraction is currently advancing.
~~~~


## When set, route-progress calculations use paused_at instead of the real

Removed from `migrations/030_mmo_route_legs_and_road_encounters.sql:23`.

~~~~text
When set, route-progress calculations use paused_at instead of the real
clock as "now" (freezing position). On resume, leg_started_at is shifted
forward by exactly the pause duration and paused_at is cleared, so
progress resumes from the same fraction rather than jumping ahead or
snapping back to the pre-pause position.
~~~~


## Backfill existing in-flight raids so the progress fraction degrades

Removed from `migrations/030_mmo_route_legs_and_road_encounters.sql:30`.

~~~~text
Backfill existing in-flight raids so the progress fraction degrades
gracefully instead of erroring on a NULL leg start.
~~~~


## A road encounter is a reciprocal, time-boxed decision point between two

Removed from `migrations/030_mmo_route_legs_and_road_encounters.sql:35`.

~~~~text
A road encounter is a reciprocal, time-boxed decision point between two
marching/returning expeditions whose current position converged. Either
commander choosing "Attack" resolves it as a field battle; both choosing
"Continue" (or the deadline lapsing with no attack) resolves it as a
peaceful pass. Canonical ordering (raid_a_id < raid_b_id as text) keeps the
pending-pair unique index meaningful and gives tick passes a stable lock
order, avoiding duplicate encounters or lock-order deadlocks.
~~~~


## active_encounter_id references road_encounters, which is defined above it

Removed from `migrations/030_mmo_route_legs_and_road_encounters.sql:69`.

~~~~text
active_encounter_id references road_encounters, which is defined above it
in this same file, so the FK is added afterwards via a guarded DO block
(Postgres has no ADD CONSTRAINT IF NOT EXISTS).
~~~~
