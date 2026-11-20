# ADR 0230: Decisions from `migrations/025_spacehunt_phase7_regional_world_events.sql`

Status: Accepted

These decisions were written as comments in `migrations/025_spacehunt_phase7_regional_world_events.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Item 12: world events move from one global "weather front" (world_state

Removed from `migrations/025_spacehunt_phase7_regional_world_events.sql:10`.

~~~~text
Item 12: world events move from one global "weather front" (world_state
.active_weather, a single row) to independent per-continent events,
scoped to the same Africa/Europe/Asia/Americas quadrant scheme already
used by coordinates.region for spawn placement. A continent's active
event is simply the newest unexpired row for that continent in
world_events; the absence of one means nominal/clear conditions.

world_events already existed (created in an earlier phase) but was
dead code - a cleanup tick phase deleted expired rows from it, but
nothing ever inserted any. This migration is what finally wires it up.
~~~~
