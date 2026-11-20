
CREATE TABLE IF NOT EXISTS encampment_discoveries (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	observer_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	target_encampment_id UUID REFERENCES encampments(id) ON DELETE CASCADE,
	target_key VARCHAR(100),
	discovery_method VARCHAR(50) NOT NULL,
	discovered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT encampment_discoveries_one_target CHECK (
		(target_encampment_id IS NOT NULL AND target_key IS NULL)
		OR (target_encampment_id IS NULL AND target_key IS NOT NULL)
	)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_encampment_discoveries_encampment_target
	ON encampment_discoveries(observer_encampment_id, target_encampment_id)
	WHERE target_encampment_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_encampment_discoveries_system_target
	ON encampment_discoveries(observer_encampment_id, target_key)
	WHERE target_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_encampment_discoveries_observer_recent
	ON encampment_discoveries(observer_encampment_id, last_seen_at DESC);

ALTER TABLE raids ADD COLUMN IF NOT EXISTS origin_x INT;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS origin_y INT;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS destination_x INT;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS destination_y INT;
ALTER TABLE raids ADD COLUMN IF NOT EXISTS origin_region VARCHAR(50);
ALTER TABLE raids ADD COLUMN IF NOT EXISTS destination_region VARCHAR(50);
ALTER TABLE raids ADD COLUMN IF NOT EXISTS radar_alert_sent_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_raids_marching_radar_pending
	ON raids(resolve_time)
	WHERE state = 'marching' AND defender_id IS NOT NULL AND radar_alert_sent_at IS NULL;
