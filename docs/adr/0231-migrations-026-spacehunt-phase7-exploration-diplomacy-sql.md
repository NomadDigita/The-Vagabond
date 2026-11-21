# ADR 0231: Decisions from `migrations/026_spacehunt_phase7_exploration_diplomacy.sql`

Status: Accepted

These decisions were written as comments in `migrations/026_spacehunt_phase7_exploration_diplomacy.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Item 10: World Exploration. Sites rotate in per continent (same

Removed from `migrations/026_spacehunt_phase7_exploration_diplomacy.sql:10`.

~~~~text
Item 10: World Exploration. Sites rotate in per continent (same
pattern as world_events) and are claimed first-come-first-served.
~~~~


## Item 11: Diplomacy. Mirrors clan_wars's clan_a/clan_b shape, for

Removed from `migrations/026_spacehunt_phase7_exploration_diplomacy.sql:33`.

~~~~text
Item 11: Diplomacy. Mirrors clan_wars's clan_a/clan_b shape, for
peaceful pacts instead of conflicts. Only 'active' status blocks raids
(see combat.go's launch check) - a Clan King must accept a 'pending'
proposal first.
~~~~
