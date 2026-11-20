# ADR 0215: Decisions from `migrations/003_spacehunt_slice1_fleet_and_research.sql`

Status: Accepted

These decisions were written as comments in `migrations/003_spacehunt_slice1_fleet_and_research.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## 2. Defense Grid turrets + utility structures. These all live in the

Removed from `migrations/003_spacehunt_slice1_fleet_and_research.sql:21`.

~~~~text
2. Defense Grid turrets + utility structures. These all live in the
generic `modules` table (encampment_id, type, level) which already
existed for tent/scrap_heap/generator - no new columns needed here,
just new `type` string values used by the app:
'light_laser', 'heavy_laser', 'gauss_cannon', 'ion_cannon',
'plasma_turret', 'anti_missile', 'warehouse'
~~~~
