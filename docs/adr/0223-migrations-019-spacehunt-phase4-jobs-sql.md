# ADR 0223: Decisions from `migrations/019_spacehunt_phase4_jobs.sql`

Status: Accepted

These decisions were written as comments in `migrations/019_spacehunt_phase4_jobs.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Commands added:

Removed from `migrations/019_spacehunt_phase4_jobs.sql:15`.

~~~~text
Commands added:
/newjobhyperspeed - cuts your nearest active mission's remaining time
in half (300 Electricity)
/newjobextendplanet - permanently +1000 storage cap, escalating Metal/
Crystal cost each time (folds into resource.go's storageCap formula)
/newjobteleport - relocate to a fresh random coordinate (1000
Electricity, 24h cooldown)
/newjoborbitalmaneuver - +30% defense rating for 2 hours (400
Electricity) - folds into resolveRaidCombats' defenseRatingModifier
/newjobrepairunits - instant +5 Soldiers (200 Scrap)
/newjobrepairbuildings - cuts your active building upgrade's remaining
time in half (150 Scrap)
/newjobgathersunlight - instant +150 Electricity (30 min cooldown)
/newjobmanualscan, /newjobautoscan, /newjobadvancedscan,
/newjobpublishtrade - command-name-parity aliases pointing to the
existing /scout, /autoscan, satellite recon, and Market Exchange
features respectively
~~~~
