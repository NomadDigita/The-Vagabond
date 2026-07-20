-- ==============================================================================
-- THE VAGABOND - MMO LOGISTICS HARDENING: STAGED TRANSPORTS + FORCE RECOVERY
-- (029_mmo_transport_staging_and_force_recovery.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- Transport vehicles must be in a campaign column, not merely owned at home.
-- Matching idempotent statements are in cmd/bot/main.go.
-- ==============================================================================

ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS haulers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS tankers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS cargo_mk1 INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS cargo_mk2 INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS cargo_mk3 INT DEFAULT 0;

ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS ships_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS jets_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS nukes_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS haulers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS tankers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS cargo_mk1_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS cargo_mk2_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS cargo_mk3_mobilized INT DEFAULT 0;
