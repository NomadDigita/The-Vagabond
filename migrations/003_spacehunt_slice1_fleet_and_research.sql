-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: RESEARCH TREE, DEFENSE GRID, FLEET
-- (003_spacehunt_slice1_fleet_and_research.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup (CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS), so
-- you do NOT need to run this file by hand for the bot itself to work.
-- It's kept here purely as a readable, versioned reference of what changed,
-- and so you can run it directly in the Supabase SQL editor if you ever
-- want to inspect/verify the schema state without booting the bot.
-- ==============================================================================

-- 1. Full 7-node Research Tree (Technology, Production, Integrity, Shields,
--    Intelligence, Thrusters, Weapons)
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS production_tech_lvl INT DEFAULT 1;
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS integrity_tech_lvl INT DEFAULT 1;
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS intel_tech_lvl INT DEFAULT 1;
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS speed_tech_lvl INT DEFAULT 1;

-- 2. Defense Grid turrets + utility structures. These all live in the
--    generic `modules` table (encampment_id, type, level) which already
--    existed for tent/scrap_heap/generator - no new columns needed here,
--    just new `type` string values used by the app:
--      'light_laser', 'heavy_laser', 'gauss_cannon', 'ion_cannon',
--      'plasma_turret', 'anti_missile', 'warehouse'

-- 3. Destroyer & Bomber fleet units
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;

ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;

ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS destroyers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS bombers_mobilized INT DEFAULT 0;
