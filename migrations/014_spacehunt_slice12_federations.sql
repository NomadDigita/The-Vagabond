-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: FEDERATIONS (GUILD-OF-GUILDS)
-- (014_spacehunt_slice12_federations.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

CREATE TABLE IF NOT EXISTS federations (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name VARCHAR(255) UNIQUE NOT NULL,
	icon VARCHAR(10) DEFAULT '🌐',
	description TEXT DEFAULT '',
	founder_clan_id UUID REFERENCES clans(id) ON DELETE SET NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE clans ADD COLUMN IF NOT EXISTS federation_id UUID REFERENCES federations(id) ON DELETE SET NULL;

-- Commands: /federations (ranking), /federation (your own), /fed_found
-- [name] (5000 Crystal, King-only), /fed_join [name] (King-only),
-- /fed_leave (King-only). Matches SpaceHunt's Federations feature -
-- a tier above individual Clans/Guilds.
