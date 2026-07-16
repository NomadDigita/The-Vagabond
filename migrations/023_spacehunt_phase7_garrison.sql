-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 7 HERO COMMANDER / MANUAL GARRISON
-- (023_spacehunt_phase7_garrison.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs the statements below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- Lets a player manually lock a portion of their Soldiers/Mechs into a
-- permanent home garrison. Garrisoned units:
--   - Still count fully toward defense (workshop_inventory.soldiers/mechs
--     is unchanged - garrison is a *reservation*, not a separate pool).
--   - Are excluded from the *draftable* total shown/enforced when staging
--     a campaign (see combat.go availSoldiers/availMechs), so a player
--     can't accidentally send their entire home defense out on a raid.
--   - Can be withdrawn (un-reserved) at any time via the Garrison panel.
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS garrisoned_soldiers INT DEFAULT 0;
ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS garrisoned_mechs INT DEFAULT 0;
