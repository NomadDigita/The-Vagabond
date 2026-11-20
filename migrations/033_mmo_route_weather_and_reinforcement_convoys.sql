
CREATE TABLE IF NOT EXISTS route_incidents (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	incident_type VARCHAR(20) NOT NULL,
	severity INT NOT NULL DEFAULT 1,
	location_x INT NOT NULL,
	location_y INT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	cleared_at TIMESTAMP WITH TIME ZONE NOT NULL,
	resolved BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_route_incidents_pending_clear ON route_incidents(cleared_at) WHERE resolved = FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS uq_route_incidents_active_raid ON route_incidents(raid_id) WHERE resolved = FALSE;

ALTER TABLE raids ADD COLUMN IF NOT EXISTS active_incident_id UUID;

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

CREATE TABLE IF NOT EXISTS supply_convoys (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	home_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	target_raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
	state VARCHAR(20) NOT NULL DEFAULT 'marching',
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
CREATE UNIQUE INDEX IF NOT EXISTS uq_supply_convoys_active_target ON supply_convoys(target_raid_id) WHERE state = 'marching';
