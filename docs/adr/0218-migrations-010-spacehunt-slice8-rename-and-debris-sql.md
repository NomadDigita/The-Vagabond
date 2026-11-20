# ADR 0218: Decisions from `migrations/010_spacehunt_slice8_rename_and_debris.sql`

Status: Accepted

These decisions were written as comments in `migrations/010_spacehunt_slice8_rename_and_debris.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## 1. Battle debris now loots Metal + Crystal (matching SpaceHunt's exact

Removed from `migrations/010_spacehunt_slice8_rename_and_debris.sql:10`.

~~~~text
1. Battle debris now loots Metal + Crystal (matching SpaceHunt's exact
"Battle debris: <metal> <crystal>" battle report format) alongside
the existing Scrap loot.
~~~~


## 2. Outpost renaming (/name) needs no new schema - it writes directly to

Removed from `migrations/010_spacehunt_slice8_rename_and_debris.sql:16`.

~~~~text
2. Outpost renaming (/name) needs no new schema - it writes directly to
the existing encampments.name column, gated by a resource cost check
in application code (1000 Crystal + $500).
~~~~
