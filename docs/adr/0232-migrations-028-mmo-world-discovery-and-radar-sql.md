# ADR 0232: Decisions from `migrations/028_mmo_world_discovery_and_radar.sql`

Status: Accepted

These decisions were written as comments in `migrations/028_mmo_world_discovery_and_radar.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## ==============================================================================

Removed from `migrations/028_mmo_world_discovery_and_radar.sql:1`.

~~~~text
==============================================================================
THE VAGABOND - MMO LIVING WORLD: DISCOVERY GATES + RADAR PROXIMITY
(028_mmo_world_discovery_and_radar.sql)
DB Engine: PostgreSQL (Supabase)

Server-only gameplay tables. The bot uses a direct PostgreSQL connection;
this migration deliberately grants no Data API roles. If a future client
consumes these tables, enable RLS and add narrowly scoped policies first.
Matching idempotent statements are in cmd/bot/main.go for existing installs.
==============================================================================
~~~~


## A discovery is directional: observer -> target. A target is either an

Removed from `migrations/028_mmo_world_discovery_and_radar.sql:12`.

~~~~text
A discovery is directional: observer -> target. A target is either an
encampment or a named system target (currently ai_drone_nest), never both.
~~~~


## The current raid schema stays compatible, but records just enough immutable

Removed from `migrations/028_mmo_world_discovery_and_radar.sql:37`.

~~~~text
The current raid schema stays compatible, but records just enough immutable
route/radar state for proximity warnings and future route-event phases.
~~~~
