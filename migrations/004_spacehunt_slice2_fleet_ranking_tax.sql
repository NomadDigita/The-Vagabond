-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: FLEET, RANKING, DAILY TAX LAW
-- (004_spacehunt_slice2_fleet_ranking_tax.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- 1. Destroyer, Bomber, Scout Walker, Battlecruiser fleet units
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS scouts INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS battlecruisers INT DEFAULT 0;

ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS battlecruisers INT DEFAULT 0;

ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS destroyers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS bombers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS battlecruisers_mobilized INT DEFAULT 0;

-- Note: Scout Walkers are garrison/recon units only (not draftable into
-- raids) so they have no campaign_drafts/raid_forces column - only
-- workshop_inventory.

-- 2. Daily Tax Law
CREATE TABLE IF NOT EXISTS tax_law (
	id INT PRIMARY KEY DEFAULT 1,
	tax_rate_percent INT DEFAULT 5,
	last_collected_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT single_row CHECK (id = 1)
);
INSERT INTO tax_law (id, tax_rate_percent) VALUES (1, 5) ON CONFLICT (id) DO NOTHING;

-- 3. Global Ranking board (/ranking) needs no new schema - it computes
--    score live from existing encampments/modules/resources/workshop_inventory
--    via internal/game/scoring.ScoreExpr.
