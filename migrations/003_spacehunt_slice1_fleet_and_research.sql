
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS production_tech_lvl INT DEFAULT 1;
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS integrity_tech_lvl INT DEFAULT 1;
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS intel_tech_lvl INT DEFAULT 1;
ALTER TABLE research_states ADD COLUMN IF NOT EXISTS speed_tech_lvl INT DEFAULT 1;


ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;

ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS destroyers INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS bombers INT DEFAULT 0;

ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS destroyers_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS bombers_mobilized INT DEFAULT 0;
