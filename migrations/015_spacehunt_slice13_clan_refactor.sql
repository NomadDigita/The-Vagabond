-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: CLAN REFACTOR (NAMES, APPLICATIONS, REAL WARS)
-- (015_spacehunt_slice13_clan_refactor.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- Join-request/application flow: browse clans, apply, Leader accepts/rejects.
CREATE TABLE IF NOT EXISTS clan_applications (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	clan_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	status VARCHAR(50) DEFAULT 'pending',
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(clan_id, user_id)
);

-- Real, live-scored Clan Wars (48h duration, score accumulated from actual
-- raid outcomes between the two clans' members, resolved by the tick
-- engine with a shared spoils payout to the winning clan).
CREATE TABLE IF NOT EXISTS clan_wars (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	clan_a_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
	clan_b_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
	score_a DOUBLE PRECISION DEFAULT 0,
	score_b DOUBLE PRECISION DEFAULT 0,
	status VARCHAR(50) DEFAULT 'active',
	started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	ends_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Custom naming (/clan_create [name]) and paid renaming (/clan_rename
-- [name], 800 Crystal, Leader-only) need no new schema - they write
-- directly to the existing clans.name column.
