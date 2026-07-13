-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 4 - JOB COMMANDS
-- (019_spacehunt_phase4_jobs.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

ALTER TABLE encampments ADD COLUMN IF NOT EXISTS extension_lvl INT DEFAULT 0;
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS orbital_buff_until TIMESTAMP WITH TIME ZONE;
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS last_teleport_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS last_sunlight_at TIMESTAMP WITH TIME ZONE;

-- Commands added:
-- /newjobhyperspeed - cuts your nearest active mission's remaining time
--   in half (300 Electricity)
-- /newjobextendplanet - permanently +1000 storage cap, escalating Metal/
--   Crystal cost each time (folds into resource.go's storageCap formula)
-- /newjobteleport - relocate to a fresh random coordinate (1000
--   Electricity, 24h cooldown)
-- /newjoborbitalmaneuver - +30% defense rating for 2 hours (400
--   Electricity) - folds into resolveRaidCombats' defenseRatingModifier
-- /newjobrepairunits - instant +5 Soldiers (200 Scrap)
-- /newjobrepairbuildings - cuts your active building upgrade's remaining
--   time in half (150 Scrap)
-- /newjobgathersunlight - instant +150 Electricity (30 min cooldown)
-- /newjobmanualscan, /newjobautoscan, /newjobadvancedscan,
--   /newjobpublishtrade - command-name-parity aliases pointing to the
--   existing /scout, /autoscan, satellite recon, and Market Exchange
--   features respectively
