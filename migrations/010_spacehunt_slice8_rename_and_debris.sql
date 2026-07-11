-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: OUTPOST RENAME + MULTI-RESOURCE BATTLE DEBRIS
-- (010_spacehunt_slice8_rename_and_debris.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- 1. Battle debris now loots Metal + Crystal (matching SpaceHunt's exact
--    "Battle debris: <metal> <crystal>" battle report format) alongside
--    the existing Scrap loot.
ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_metal DOUBLE PRECISION DEFAULT 0.00;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_crystal DOUBLE PRECISION DEFAULT 0.00;

-- 2. Outpost renaming (/name) needs no new schema - it writes directly to
--    the existing encampments.name column, gated by a resource cost check
--    in application code (1000 Crystal + $500).
