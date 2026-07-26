-- ==============================================================================
-- THE VAGABOND - MMO LIVING WORLD: ROAD ENCOUNTERS
-- (030_mmo_road_encounters.sql)
-- DB Engine: PostgreSQL (Supabase)
-- ==============================================================================

CREATE TABLE IF NOT EXISTS road_encounters (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	primary_raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	secondary_raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	region VARCHAR(50) NOT NULL,
	x INT NOT NULL,
	y INT NOT NULL,
	primary_route_state VARCHAR(20) NOT NULL DEFAULT 'marching',
	secondary_route_state VARCHAR(20) NOT NULL DEFAULT 'marching',
	primary_decision VARCHAR(20) NOT NULL DEFAULT 'pending',
	secondary_decision VARCHAR(20) NOT NULL DEFAULT 'pending',
	state VARCHAR(20) NOT NULL DEFAULT 'pending',
	decision_deadline TIMESTAMP WITH TIME ZONE NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	resolved_at TIMESTAMP WITH TIME ZONE,
	CONSTRAINT road_encounters_distinct_raids CHECK (primary_raid_id <> secondary_raid_id),
	CONSTRAINT road_encounters_primary_decision CHECK (primary_decision IN ('pending', 'continue', 'attack')),
	CONSTRAINT road_encounters_secondary_decision CHECK (secondary_decision IN ('pending', 'continue', 'attack')),
	CONSTRAINT road_encounters_primary_route_state CHECK (primary_route_state IN ('marching', 'returning')),
	CONSTRAINT road_encounters_secondary_route_state CHECK (secondary_route_state IN ('marching', 'returning')),
	CONSTRAINT road_encounters_state CHECK (state IN ('pending', 'resolving', 'continued', 'resolved', 'expired'))
);

ALTER TABLE road_encounters DROP CONSTRAINT IF EXISTS road_encounters_unique_pair;
ALTER TABLE road_encounters ADD COLUMN IF NOT EXISTS primary_route_state VARCHAR(20) NOT NULL DEFAULT 'marching';
ALTER TABLE road_encounters ADD COLUMN IF NOT EXISTS secondary_route_state VARCHAR(20) NOT NULL DEFAULT 'marching';
CREATE UNIQUE INDEX IF NOT EXISTS uq_road_encounters_active_pair
	ON road_encounters(primary_raid_id, secondary_raid_id)
	WHERE state IN ('pending', 'resolving');
-- A pair may meet again on a different leg (for example outbound versus
-- returning), but cannot be repeatedly prompted while remaining adjacent on
-- the same two legs.
CREATE UNIQUE INDEX IF NOT EXISTS uq_road_encounters_route_legs
	ON road_encounters(primary_raid_id, secondary_raid_id, primary_route_state, secondary_route_state);
CREATE INDEX IF NOT EXISTS idx_road_encounters_actionable
	ON road_encounters(decision_deadline)
	WHERE state IN ('pending', 'resolving');
