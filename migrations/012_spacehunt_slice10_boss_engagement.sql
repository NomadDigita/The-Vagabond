-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: REAL WORLD BOSS ENGAGEMENT + TRADE HUB
-- (012_spacehunt_slice10_boss_engagement.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- World Bosses now retaliate for real. Each boss has a danger tier
-- (retaliation_rating, as a % of committed troops lost per engagement).
ALTER TABLE world_bosses ADD COLUMN IF NOT EXISTS retaliation_rating DOUBLE PRECISION DEFAULT 8.0;
UPDATE world_bosses SET retaliation_rating = 6.0 WHERE name = 'The Rustlord' AND retaliation_rating = 8.0;
UPDATE world_bosses SET retaliation_rating = 12.0 WHERE name = 'Scrap Titan' AND retaliation_rating = 8.0;
UPDATE world_bosses SET retaliation_rating = 22.0 WHERE name = 'Apex Wraith' AND retaliation_rating = 8.0;

-- Tracks each real engagement: marching -> retaliation -> returning ->
-- home, mirroring the same lifecycle as a real PvP raid instead of the
-- old instant-resolve model.
CREATE TABLE IF NOT EXISTS world_boss_attacks (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	boss_id UUID NOT NULL REFERENCES world_bosses(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	soldiers_committed INT DEFAULT 0,
	mechs_committed INT DEFAULT 0,
	state VARCHAR(50) DEFAULT 'marching',
	resolve_time TIMESTAMP WITH TIME ZONE NOT NULL,
	march_minutes DOUBLE PRECISION DEFAULT 8.0,
	damage_dealt DOUBLE PRECISION DEFAULT 0,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Trade Hub (economy.go) needed no new schema - it wires up the
-- previously-unreachable buy_metal/buy_crystal/buy_hydrogen conversions
-- (they existed in code but had no buttons pointing to them) and adds
-- sell_metal/sell_crystal, all against the existing resources table.
