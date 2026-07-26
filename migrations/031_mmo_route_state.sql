-- ==============================================================================
-- THE VAGABOND - MMO LIVING WORLD: ROUTE STATE
-- (031_mmo_route_state.sql)
-- DB Engine: PostgreSQL (Supabase)
-- ==============================================================================

ALTER TABLE raids ADD COLUMN IF NOT EXISTS movement_state VARCHAR(30) NOT NULL DEFAULT 'moving';
ALTER TABLE raids ADD COLUMN IF NOT EXISTS pause_reason TEXT;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS next_route_event_at TIMESTAMP WITH TIME ZONE;
-- Route progress is frozen whenever a column is paused.  The prior
-- ETA-only model could not distinguish a pause from a longer journey and
-- would move a delayed expedition backwards along its route.
ALTER TABLE raids ADD COLUMN IF NOT EXISTS route_progress DOUBLE PRECISION;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS route_progress_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS route_leg_minutes DOUBLE PRECISION;
UPDATE raids
SET route_progress = CASE WHEN state = 'returning' THEN 1.0 ELSE 0.0 END
WHERE route_progress IS NULL;
ALTER TABLE raids ALTER COLUMN route_progress SET DEFAULT 0.0;
ALTER TABLE raids ALTER COLUMN route_progress SET NOT NULL;
DO $$
BEGIN
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'raids_movement_state_valid') THEN
		ALTER TABLE raids ADD CONSTRAINT raids_movement_state_valid
			CHECK (movement_state IN ('moving', 'encounter_pending', 'encounter_battle', 'battle_recovery', 'weather_paused', 'supply_paused'));
	END IF;
	IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'raids_route_progress_range') THEN
		ALTER TABLE raids ADD CONSTRAINT raids_route_progress_range
			CHECK (route_progress >= 0.0 AND route_progress <= 1.0);
	END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_raids_active_movement
	ON raids(state, movement_state, resolve_time)
	WHERE state IN ('marching', 'returning');
