# ADR 0220: Decisions from `migrations/012_spacehunt_slice10_boss_engagement.sql`

Status: Accepted

These decisions were written as comments in `migrations/012_spacehunt_slice10_boss_engagement.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## World Bosses now retaliate for real. Each boss has a danger tier

Removed from `migrations/012_spacehunt_slice10_boss_engagement.sql:10`.

~~~~text
World Bosses now retaliate for real. Each boss has a danger tier
(retaliation_rating, as a % of committed troops lost per engagement).
~~~~


## Tracks each real engagement: marching -> retaliation -> returning ->

Removed from `migrations/012_spacehunt_slice10_boss_engagement.sql:17`.

~~~~text
Tracks each real engagement: marching -> retaliation -> returning ->
home, mirroring the same lifecycle as a real PvP raid instead of the
old instant-resolve model.
~~~~


## Trade Hub (economy.go) needed no new schema - it wires up the

Removed from `migrations/012_spacehunt_slice10_boss_engagement.sql:34`.

~~~~text
Trade Hub (economy.go) needed no new schema - it wires up the
previously-unreachable buy_metal/buy_crystal/buy_hydrogen conversions
(they existed in code but had no buttons pointing to them) and adds
sell_metal/sell_crystal, all against the existing resources table.
~~~~
