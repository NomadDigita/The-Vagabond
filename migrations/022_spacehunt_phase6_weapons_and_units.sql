-- Phase 6: Engage-weapon turret differentiation + remaining new units
-- (Liberator, Observer, Wraith, Piercing Missile, Guardian, Cargo Ship
-- Mk I/II/III) and their deconstruct/raid-draft plumbing.

-- Workshop inventory: every new craftable unit gets its own column,
-- matching the established per-unit-type pattern (soldiers, mechs, etc).
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS liberators INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS observers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS wraiths INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS piercing_missiles INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS guardians INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS cargo_mk1 INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS cargo_mk2 INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS cargo_mk3 INT DEFAULT 0;

-- Campaign draft staging + raid_forces mobilization: only Liberator and
-- Wraith are raid-draftable offensive units (Observer/Guardian are
-- garrison-only utility/defense units, Piercing Missile is a Silo-launched
-- strike weapon like Nukes, and Cargo Ship tiers are logistics vehicles
-- that ride the same buggy/ship/jet/hauler/tanker/rig craft pipeline
-- rather than the combat draft board).
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS liberators INT DEFAULT 0;
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS wraiths INT DEFAULT 0;

ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS liberators_mobilized INT DEFAULT 0;
ALTER TABLE raid_forces ADD COLUMN IF NOT EXISTS wraiths_mobilized INT DEFAULT 0;
