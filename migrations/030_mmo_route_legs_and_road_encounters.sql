
ALTER TABLE raids ADD COLUMN IF NOT EXISTS leg_started_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS leg_total_minutes DOUBLE PRECISION;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS movement_state VARCHAR(30) NOT NULL DEFAULT 'moving';
ALTER TABLE raids ADD COLUMN IF NOT EXISTS paused_remaining_minutes DOUBLE PRECISION;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_encounter_id UUID;

ALTER TABLE raids ADD COLUMN IF NOT EXISTS paused_at TIMESTAMP WITH TIME ZONE;

UPDATE raids SET leg_started_at = COALESCE(leg_started_at, created_at, CURRENT_TIMESTAMP) WHERE leg_started_at IS NULL;
UPDATE raids SET leg_total_minutes = COALESCE(leg_total_minutes, base_march_minutes, 15.0) WHERE leg_total_minutes IS NULL;

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
