-- ==============================================================================
-- THE VAGABOND - MMO LIVING WORLD: ROAD-VS-BASE ENCOUNTERS
-- (034_mmo_road_base_encounters.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- Completes MMO_WORLD_EVOLUTION_PLAN.md Phase 4 milestone 2, which shipped
-- expedition-vs-expedition road encounters (030) but not the "expeditions
-- and bases" half: a moving column passing near a passive player base only
-- ever produced a Phase 2 discovery, never a forced Attack/Continue window.
--
-- Deliberately a separate table from road_encounters rather than widening
-- it to allow a NULL raid_b_id: road_encounters' ordering/uniqueness
-- constraints (raid_a_id < raid_b_id, both NOT NULL) assume two raids, and
-- the already-verified expedition-vs-expedition path should not be
-- disturbed to accommodate a one-sided (raid vs. stationary base) case.
-- ==============================================================================

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

-- Only one live decision window per (expedition, base) pair at a time.
CREATE UNIQUE INDEX IF NOT EXISTS uq_road_base_encounters_pending_pair
	ON road_base_encounters(raid_id, encampment_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_road_base_encounters_pending_deadline
	ON road_base_encounters(response_deadline) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_road_base_encounters_raid_recent
	ON road_base_encounters(raid_id, created_at DESC);

ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_base_encounter_id UUID;

-- Same guarded-FK pattern as active_encounter_id/road_encounters in 030,
-- since road_base_encounters is defined above this ALTER in the same file.
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
