-- ==============================================================================
-- THE VAGABOND - MMO LIVING WORLD: ROUTE LEGS + ROAD ENCOUNTERS
-- (030_mmo_route_legs_and_road_encounters.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- Implements MMO_WORLD_EVOLUTION_PLAN.md Phase 3 (geographic route legs) and
-- Phase 4 (road contacts / field battles). Matching idempotent statements are
-- duplicated in cmd/bot/main.go for existing installs.
-- ==============================================================================

-- Each raid's "leg" is the current uninterrupted movement segment (the
-- outbound march or the return march). leg_started_at/leg_total_minutes let
-- the engine compute a stable 0..1 route-progress fraction for the CURRENT
-- leg without being confused by round-trip resolve_time reuse across the
-- marching -> engaged -> returning state machine. movement_state gates
-- whether that fraction is currently advancing.
ALTER TABLE raids ADD COLUMN IF NOT EXISTS leg_started_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS leg_total_minutes DOUBLE PRECISION;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS movement_state VARCHAR(30) NOT NULL DEFAULT 'moving';
ALTER TABLE raids ADD COLUMN IF NOT EXISTS paused_remaining_minutes DOUBLE PRECISION;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_encounter_id UUID;

-- When set, route-progress calculations use paused_at instead of the real
-- clock as "now" (freezing position). On resume, leg_started_at is shifted
-- forward by exactly the pause duration and paused_at is cleared, so
-- progress resumes from the same fraction rather than jumping ahead or
-- snapping back to the pre-pause position.
ALTER TABLE raids ADD COLUMN IF NOT EXISTS paused_at TIMESTAMP WITH TIME ZONE;

-- Backfill existing in-flight raids so the progress fraction degrades
-- gracefully instead of erroring on a NULL leg start.
UPDATE raids SET leg_started_at = COALESCE(leg_started_at, created_at, CURRENT_TIMESTAMP) WHERE leg_started_at IS NULL;
UPDATE raids SET leg_total_minutes = COALESCE(leg_total_minutes, base_march_minutes, 15.0) WHERE leg_total_minutes IS NULL;

-- A road encounter is a reciprocal, time-boxed decision point between two
-- marching/returning expeditions whose current position converged. Either
-- commander choosing "Attack" resolves it as a field battle; both choosing
-- "Continue" (or the deadline lapsing with no attack) resolves it as a
-- peaceful pass. Canonical ordering (raid_a_id < raid_b_id as text) keeps the
-- pending-pair unique index meaningful and gives tick passes a stable lock
-- order, avoiding duplicate encounters or lock-order deadlocks.
CREATE TABLE IF NOT EXISTS road_encounters (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	raid_a_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	raid_b_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	location_x INT NOT NULL,
	location_y INT NOT NULL,
	status VARCHAR(20) NOT NULL DEFAULT 'pending',
	decision_a VARCHAR(20),
	decision_b VARCHAR(20),
	outcome VARCHAR(20),
	winner_raid_id UUID REFERENCES raids(id) ON DELETE SET NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	response_deadline TIMESTAMP WITH TIME ZONE NOT NULL,
	resolved_at TIMESTAMP WITH TIME ZONE,
	CONSTRAINT road_encounters_distinct_parties CHECK (raid_a_id <> raid_b_id),
	CONSTRAINT road_encounters_ordered_pair CHECK (raid_a_id::text < raid_b_id::text)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_road_encounters_pending_pair
	ON road_encounters(raid_a_id, raid_b_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_road_encounters_pending_deadline
	ON road_encounters(response_deadline) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_road_encounters_raid_a_recent
	ON road_encounters(raid_a_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_road_encounters_raid_b_recent
	ON road_encounters(raid_b_id, created_at DESC);

-- active_encounter_id references road_encounters, which is defined above it
-- in this same file, so the FK is added afterwards via a guarded DO block
-- (Postgres has no ADD CONSTRAINT IF NOT EXISTS).
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint WHERE conname = 'raids_active_encounter_id_fkey'
	) THEN
		ALTER TABLE raids
			ADD CONSTRAINT raids_active_encounter_id_fkey
			FOREIGN KEY (active_encounter_id) REFERENCES road_encounters(id) ON DELETE SET NULL;
	END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_raids_moving_route_scan
	ON raids(state)
	WHERE state IN ('marching', 'returning') AND movement_state = 'moving';
