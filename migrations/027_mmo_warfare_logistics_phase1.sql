-- ==============================================================================
-- THE VAGABOND — MMO WARFARE LOGISTICS: PHASE 1
-- (027_mmo_warfare_logistics_phase1.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go runs matching statements automatically on bot startup. This file
-- is the annotated standalone reference for operators and future developers.
-- ==============================================================================

ALTER TABLE raids ADD COLUMN IF NOT EXISTS attacker_electricity DOUBLE PRECISION DEFAULT 100.0;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS attacker_logistics DOUBLE PRECISION DEFAULT 100.0;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_rations DOUBLE PRECISION DEFAULT 0.0;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_electricity DOUBLE PRECISION DEFAULT 0.0;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_hydrogen DOUBLE PRECISION DEFAULT 0.0;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_neuro_cores DOUBLE PRECISION DEFAULT 0.0;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_dollars DOUBLE PRECISION DEFAULT 0.0;
