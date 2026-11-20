# ADR 0237: Decisions from `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql`

Status: Accepted

These decisions were written as comments in `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## ==============================================================================

Removed from `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql:1`.

~~~~text
==============================================================================
THE VAGABOND - MMO LIVING WORLD: WEATHER ROUTE INCIDENTS + REINFORCEMENT
CONVOYS (033_mmo_route_weather_and_reinforcement_convoys.sql)
DB Engine: PostgreSQL (Supabase)

Implements MMO_WORLD_EVOLUTION_PLAN.md Phase 5. Reuses the Phase 3/4
pause/resume mechanism (paused_at + leg_started_at shift) rather than
inventing a second pause system - a weather-camped column and an
encounter-frozen column are paused for the same structural reason
(something external is blocking travel), they just differ in what
blocked them and what un-blocks them.
==============================================================================
~~~~


## A route incident is a local weather event affecting one specific moving

Removed from `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql:14`.

~~~~text
A route incident is a local weather event affecting one specific moving
expedition (a flash flood, a storm, a heatwave) - distinct from the
existing continent-wide world_events table (internal/engine/world),
which this migration deliberately does not touch or duplicate. Route
incidents instead READ world_events as an input: an active continent-wide
event raises the odds of a matching local incident on a column currently
travelling through that continent (see evaluateRouteWeatherIncidents).
~~~~


## 1 (minor, ~12h) .. 3 (severe, ~36h)

Removed from `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql:25`.

~~~~text
1 (minor, ~12h) .. 3 (severe, ~36h)
~~~~


## Phase 5 milestone 4: rations/ammo depletion halts a column outright

Removed from `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql:37`.

~~~~text
Phase 5 milestone 4: rations/ammo depletion halts a column outright
(nothing to fight or march with), but electricity/logistics depletion
alone is softer - "high tech" (mech bonuses, capital-unit tech
multipliers) goes offline first, and only escalates to a full halt if
power stays out past a short grace period.
~~~~


## A supply convoy is a dedicated resupply expedition a commander dispatches

Removed from `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql:55`.

~~~~text
A supply convoy is a dedicated resupply expedition a commander dispatches
from their home base toward one of their OWN stranded (awaiting
reinforcement) columns. It is not an instant refill: it consumes real
transport units and real resources, takes real travel time proportional
to the distance from home to the stranded column's frozen position, and
can be lost if it is not accompanied by escorting combat units and gets
ambushed on the road in a future pass (see Known design assumptions).
~~~~


## Only one convoy in flight per stranded column at a time, so a commander

Removed from `migrations/033_mmo_route_weather_and_reinforcement_convoys.sql:82`.

~~~~text
Only one convoy in flight per stranded column at a time, so a commander
can't stack redundant resupply runs.
~~~~
