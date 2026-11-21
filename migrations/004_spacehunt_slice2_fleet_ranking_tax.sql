
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS scouts INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS battlecruisers INT DEFAULT 0;

ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS battlecruisers INT DEFAULT 0;

ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS destroyers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS bombers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS battlecruisers_mobilized INT DEFAULT 0;


CREATE TABLE IF NOT EXISTS tax_law (
	id INT PRIMARY KEY DEFAULT 1,
	tax_rate_percent INT DEFAULT 5,
	last_collected_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT single_row CHECK (id = 1)
);
INSERT INTO tax_law (id, tax_rate_percent) VALUES (1, 5) ON CONFLICT (id) DO NOTHING;

