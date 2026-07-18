-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 7 EXPLORATION + DIPLOMACY
-- (026_spacehunt_phase7_exploration_diplomacy.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs the statements below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- Item 10: World Exploration. Sites rotate in per continent (same
-- pattern as world_events) and are claimed first-come-first-served.
CREATE TABLE IF NOT EXISTS exploration_sites (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	continent VARCHAR(50) NOT NULL,
	site_name VARCHAR(255) NOT NULL,
	site_type VARCHAR(50) NOT NULL,
	reward_type VARCHAR(50) NOT NULL,
	reward_amount DOUBLE PRECISION NOT NULL,
	expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
	claimed_by UUID REFERENCES encampments(id) ON DELETE SET NULL,
	claimed_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_exploration_sites_continent ON exploration_sites(continent, claimed_by, expires_at);

CREATE TABLE IF NOT EXISTS exploration_dispatches (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	site_id UUID NOT NULL UNIQUE REFERENCES exploration_sites(id) ON DELETE CASCADE,
	encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	resolve_time TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Item 11: Diplomacy. Mirrors clan_wars's clan_a/clan_b shape, for
-- peaceful pacts instead of conflicts. Only 'active' status blocks raids
-- (see combat.go's launch check) - a Clan King must accept a 'pending'
-- proposal first.
CREATE TABLE IF NOT EXISTS clan_diplomacy (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	clan_a_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
	clan_b_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
	pact_type VARCHAR(50) NOT NULL,
	status VARCHAR(50) DEFAULT 'pending',
	proposed_by BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	responded_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_clan_diplomacy_clans ON clan_diplomacy(clan_a_id, clan_b_id, status);
