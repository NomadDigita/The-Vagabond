-- ==============================================================================
-- THE VAGABOND - MMO LIVING WORLD: WEATHER ROUTE INCIDENTS + REINFORCEMENT
-- CONVOYS (031_mmo_route_weather_and_reinforcement_convoys.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- Implements MMO_WORLD_EVOLUTION_PLAN.md Phase 5. Reuses the Phase 3/4
-- pause/resume mechanism (paused_at + leg_started_at shift) rather than
-- inventing a second pause system - a weather-camped column and an
-- encounter-frozen column are paused for the same structural reason
-- (something external is blocking travel), they just differ in what
-- blocked them and what un-blocks them.
-- ==============================================================================

-- A route incident is a local weather event affecting one specific moving
-- expedition (a flash flood, a storm, a heatwave) - distinct from the
-- existing continent-wide world_events table (internal/engine/world),
-- which this migration deliberately does not touch or duplicate. Route
-- incidents instead READ world_events as an input: an active continent-wide
-- event raises the odds of a matching local incident on a column currently
-- travelling through that continent (see evaluateRouteWeatherIncidents).
CREATE TABLE IF NOT EXISTS route_incidents (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	incident_type VARCHAR(20) NOT NULL, -- flood, storm, heatwave
	severity INT NOT NULL DEFAULT 1,    -- 1 (minor, ~12h) .. 3 (severe, ~36h)
	location_x INT NOT NULL,
	location_y INT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	cleared_at TIMESTAMP WITH TIME ZONE NOT NULL,
	resolved BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_route_incidents_pending_clear ON route_incidents(cleared_at) WHERE resolved = FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS uq_route_incidents_active_raid ON route_incidents(raid_id) WHERE resolved = FALSE;

ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_incident_id UUID;

-- Phase 5 milestone 4: rations/ammo depletion halts a column outright
-- (nothing to fight or march with), but electricity/logistics depletion
-- alone is softer - "high tech" (mech bonuses, capital-unit tech
-- multipliers) goes offline first, and only escalates to a full halt if
-- power stays out past a short grace period.
ALTER TABLE raids ADD COLUMN IF NOT EXISTS high_tech_offline BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS power_outage_ticks INT NOT NULL DEFAULT 0;
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint WHERE conname = 'raids_active_incident_id_fkey'
	) THEN
		ALTER TABLE raids
			ADD CONSTRAINT raids_active_incident_id_fkey
			FOREIGN KEY (active_incident_id) REFERENCES route_incidents(id) ON DELETE SET NULL;
	END IF;
END $$;

-- A supply convoy is a dedicated resupply expedition a commander dispatches
-- from their home base toward one of their OWN stranded (awaiting
-- reinforcement) columns. It is not an instant refill: it consumes real
-- transport units and real resources, takes real travel time proportional
-- to the distance from home to the stranded column's frozen position, and
-- can be lost if it is not accompanied by escorting combat units and gets
-- ambushed on the road in a future pass (see Known design assumptions).
CREATE TABLE IF NOT EXISTS supply_convoys (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	home_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	target_raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	state VARCHAR(20) NOT NULL DEFAULT 'marching', -- marching, delivered, failed, recalled
	rations_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
	ammo_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
	electricity_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
	logistics_carried DOUBLE PRECISION NOT NULL DEFAULT 0,
	haulers_committed INT NOT NULL DEFAULT 0,
	tankers_committed INT NOT NULL DEFAULT 0,
	origin_x INT NOT NULL,
	origin_y INT NOT NULL,
	destination_x INT NOT NULL,
	destination_y INT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	resolve_time TIMESTAMP WITH TIME ZONE NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_supply_convoys_pending ON supply_convoys(resolve_time) WHERE state = 'marching';
CREATE INDEX IF NOT EXISTS idx_supply_convoys_target ON supply_convoys(target_raid_id) WHERE state = 'marching';
-- Only one convoy in flight per stranded column at a time, so a commander
-- can't stack redundant resupply runs.
CREATE UNIQUE INDEX IF NOT EXISTS uq_supply_convoys_active_target ON supply_convoys(target_raid_id) WHERE state = 'marching';
