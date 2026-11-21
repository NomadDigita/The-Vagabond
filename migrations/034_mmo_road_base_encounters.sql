
CREATE TABLE IF NOT EXISTS road_base_encounters (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	location_x INT NOT NULL,
	location_y INT NOT NULL,
	status VARCHAR(20) NOT NULL DEFAULT 'pending',
	outcome VARCHAR(20),
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	response_deadline TIMESTAMP WITH TIME ZONE NOT NULL,
	resolved_at TIMESTAMP WITH TIME ZONE,
	CONSTRAINT road_base_encounters_status CHECK (status IN ('pending', 'resolved'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_road_base_encounters_pending_pair
	ON road_base_encounters(raid_id, encampment_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_road_base_encounters_pending_deadline
	ON road_base_encounters(response_deadline) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_road_base_encounters_raid_recent
	ON road_base_encounters(raid_id, created_at DESC);

ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_base_encounter_id UUID;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint WHERE conname = 'raids_active_base_encounter_id_fkey'
	) THEN
		ALTER TABLE raids
			ADD CONSTRAINT raids_active_base_encounter_id_fkey
			FOREIGN KEY (active_base_encounter_id) REFERENCES road_base_encounters(id) ON DELETE SET NULL;
	END IF;
END $$;
