# ADR 0227: Decisions from `migrations/022_spacehunt_phase6_weapons_and_units.sql`

Status: Accepted

These decisions were written as comments in `migrations/022_spacehunt_phase6_weapons_and_units.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Workshop inventory: every new craftable unit gets its own column,

Removed from `migrations/022_spacehunt_phase6_weapons_and_units.sql:5`.

~~~~text
Workshop inventory: every new craftable unit gets its own column,
matching the established per-unit-type pattern (soldiers, mechs, etc).
~~~~


## Campaign draft staging + raid_forces mobilization: only Liberator and

Removed from `migrations/022_spacehunt_phase6_weapons_and_units.sql:16`.

~~~~text
Campaign draft staging + raid_forces mobilization: only Liberator and
Wraith are raid-draftable offensive units (Observer/Guardian are
garrison-only utility/defense units, Piercing Missile is a Silo-launched
strike weapon like Nukes, and Cargo Ship tiers are logistics vehicles
that ride the same buggy/ship/jet/hauler/tanker/rig craft pipeline
rather than the combat draft board).
~~~~
