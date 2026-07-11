-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: CORE RESOURCE CONSOLIDATION
-- (009_spacehunt_slice7_resource_consolidation.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs the equivalent of everything below
-- automatically on every bot startup (see the DO $$ blocks right after the
-- `resources` table definition). This file is a readable reference only.
-- ==============================================================================

-- Renamed to match SpaceHunt's exact 3 core resources:
--   energy   -> electricity
--   steel    -> metal
--   uranium  -> crystal
--
-- Then folded several Vagabond-specific "extra" resources into them so
-- the wasteland economy isn't needlessly fragmented, while still keeping
-- resources that serve a genuinely distinct role (Rations, Neuro Cores,
-- Hydrogen, Dollars):
--   iron, oil                -> merged into metal
--   diamond, gold, silver    -> merged into crystal
--
-- Rename (idempotent, only fires on pre-existing databases - a fresh
-- install's CREATE TABLE already uses the new names directly):
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='energy')
	   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='electricity') THEN
		ALTER TABLE resources RENAME COLUMN energy TO electricity;
	END IF;
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='steel')
	   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='metal') THEN
		ALTER TABLE resources RENAME COLUMN steel TO metal;
	END IF;
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='uranium')
	   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='crystal') THEN
		ALTER TABLE resources RENAME COLUMN uranium TO crystal;
	END IF;
END $$;

-- Merge + drop (idempotent - only fires once, since the source columns
-- won't exist on subsequent boots):
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='iron') THEN
		UPDATE resources SET metal = metal + COALESCE(iron, 0) + COALESCE(oil, 0);
		ALTER TABLE resources DROP COLUMN IF EXISTS iron;
		ALTER TABLE resources DROP COLUMN IF EXISTS oil;
	END IF;
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='diamond') THEN
		UPDATE resources SET crystal = crystal + COALESCE(diamond, 0) + COALESCE(gold, 0) + COALESCE(silver, 0);
		ALTER TABLE resources DROP COLUMN IF EXISTS diamond;
		ALTER TABLE resources DROP COLUMN IF EXISTS gold;
		ALTER TABLE resources DROP COLUMN IF EXISTS silver;
	END IF;
END $$;

-- Final resource set (7 total, down from 13):
--   scrap, rations, electricity, neuro_cores, metal, crystal, hydrogen, dollars
-- (scrap intentionally untouched - it's the core raid-loot/economy
-- currency, deeply tied to the Scrap Heap building and stolen_scrap raid
-- mechanic, and merging it would have meant rewiring that entire system
-- for no real gain)
