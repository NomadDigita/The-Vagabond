
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS garrisoned_soldiers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS garrisoned_mechs INT DEFAULT 0;
