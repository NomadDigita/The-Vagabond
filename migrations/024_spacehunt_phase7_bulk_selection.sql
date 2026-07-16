-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 7 BULK UNIT SELECTION
-- (024_spacehunt_phase7_bulk_selection.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs the statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- One "step size" per draft, cycled via a Step: x1 -> x10 -> x100 -> MAX
-- button row on the draft customizer HUD. Every existing per-unit +/-
-- button then moves this many units per tap (MAX snaps straight to the
-- draftable cap for that unit type), instead of requiring one tap per
-- unit for a large army.
ALTER TABLE campaign_drafts ADD COLUMN IF NOT EXISTS step_size INT DEFAULT 1;
