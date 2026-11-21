# ADR 0216: Decisions from `migrations/004_spacehunt_slice2_fleet_ranking_tax.sql`

Status: Accepted

These decisions were written as comments in `migrations/004_spacehunt_slice2_fleet_ranking_tax.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Note: Scout Walkers are garrison/recon units only (not draftable into

Removed from `migrations/004_spacehunt_slice2_fleet_ranking_tax.sql:24`.

~~~~text
Note: Scout Walkers are garrison/recon units only (not draftable into
raids) so they have no campaign_drafts/raid_forces column - only
workshop_inventory.
~~~~


## 3. Global Ranking board (/ranking) needs no new schema - it computes

Removed from `migrations/004_spacehunt_slice2_fleet_ranking_tax.sql:37`.

~~~~text
3. Global Ranking board (/ranking) needs no new schema - it computes
score live from existing encampments/modules/resources/workshop_inventory
via internal/game/scoring.ScoreExpr.
~~~~
