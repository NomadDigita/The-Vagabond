# ADR 0217: Decisions from `migrations/009_spacehunt_slice7_resource_consolidation.sql`

Status: Accepted

These decisions were written as comments in `migrations/009_spacehunt_slice7_resource_consolidation.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Renamed to match SpaceHunt's exact 3 core resources:

Removed from `migrations/009_spacehunt_slice7_resource_consolidation.sql:11`.

~~~~text
Renamed to match SpaceHunt's exact 3 core resources:
energy   -> electricity
steel    -> metal
uranium  -> crystal

Then folded several Vagabond-specific "extra" resources into them so
the wasteland economy isn't needlessly fragmented, while still keeping
resources that serve a genuinely distinct role (Rations, Neuro Cores,
Hydrogen, Dollars):
iron, oil                -> merged into metal
diamond, gold, silver    -> merged into crystal

Rename (idempotent, only fires on pre-existing databases - a fresh
install's CREATE TABLE already uses the new names directly):
~~~~


## Final resource set (7 total, down from 13):

Removed from `migrations/009_spacehunt_slice7_resource_consolidation.sql:58`.

~~~~text
Final resource set (7 total, down from 13):
scrap, rations, electricity, neuro_cores, metal, crystal, hydrogen, dollars
(scrap intentionally untouched - it's the core raid-loot/economy
currency, deeply tied to the Scrap Heap building and stolen_scrap raid
mechanic, and merging it would have meant rewiring that entire system
for no real gain)
~~~~
