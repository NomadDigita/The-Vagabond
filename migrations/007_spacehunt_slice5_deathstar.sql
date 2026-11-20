
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS deathstars INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS deathstars INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS deathstars_mobilized INT DEFAULT 0;

