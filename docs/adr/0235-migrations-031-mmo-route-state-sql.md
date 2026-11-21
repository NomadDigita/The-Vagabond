# ADR 0235: Decisions from `migrations/031_mmo_route_state.sql`

Status: Accepted

These decisions were written as comments in `migrations/031_mmo_route_state.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Route progress is frozen whenever a column is paused.  The prior

Removed from `migrations/031_mmo_route_state.sql:10`.

~~~~text
Route progress is frozen whenever a column is paused.  The prior
ETA-only model could not distinguish a pause from a longer journey and
would move a delayed expedition backwards along its route.
~~~~
