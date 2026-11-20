# ADR 0221: Decisions from `migrations/015_spacehunt_slice13_clan_refactor.sql`

Status: Accepted

These decisions were written as comments in `migrations/015_spacehunt_slice13_clan_refactor.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Real, live-scored Clan Wars (48h duration, score accumulated from actual

Removed from `migrations/015_spacehunt_slice13_clan_refactor.sql:20`.

~~~~text
Real, live-scored Clan Wars (48h duration, score accumulated from actual
raid outcomes between the two clans' members, resolved by the tick
engine with a shared spoils payout to the winning clan).
~~~~


## Custom naming (/clan_create [name]) and paid renaming (/clan_rename

Removed from `migrations/015_spacehunt_slice13_clan_refactor.sql:34`.

~~~~text
Custom naming (/clan_create [name]) and paid renaming (/clan_rename
[name], 800 Crystal, Leader-only) need no new schema - they write
directly to the existing clans.name column.
~~~~
