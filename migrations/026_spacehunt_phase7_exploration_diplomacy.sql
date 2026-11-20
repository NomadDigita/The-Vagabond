
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
