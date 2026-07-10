-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: DOOMSDAY RIG (DEATHSTAR)
-- (007_spacehunt_slice5_deathstar.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS deathstars INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS deathstars INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS deathstars_mobilized INT DEFAULT 0;

-- Note: hard-capped to 1 per player at the application layer
-- (factory.go HandleCraftCallback), matching its "ultimate superweapon"
-- role - no DB constraint needed since the check happens transactionally
-- before the INSERT.
