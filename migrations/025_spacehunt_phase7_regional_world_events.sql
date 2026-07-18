-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 7 REGIONAL WORLD EVENTS
-- (025_spacehunt_phase7_regional_world_events.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs the statements below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- Item 12: world events move from one global "weather front" (world_state
-- .active_weather, a single row) to independent per-continent events,
-- scoped to the same Africa/Europe/Asia/Americas quadrant scheme already
-- used by coordinates.region for spawn placement. A continent's active
-- event is simply the newest unexpired row for that continent in
-- world_events; the absence of one means nominal/clear conditions.
--
-- world_events already existed (created in an earlier phase) but was
-- dead code - a cleanup tick phase deleted expired rows from it, but
-- nothing ever inserted any. This migration is what finally wires it up.
ALTER TABLE world_events ADD COLUMN IF NOT EXISTS continent VARCHAR(50) NOT NULL DEFAULT 'Global';
CREATE INDEX IF NOT EXISTS idx_world_events_continent ON world_events(continent, expires_at);

-- world_state.active_weather is left in place (harmless legacy column) -
-- nothing reads or writes it anymore as of this migration. Every former
-- caller now resolves its own encampment's continent via
-- coordinates.region and looks up that continent's row in world_events
-- through internal/engine/world's new ActiveEventFor / ActiveEventsByContinent
-- helpers instead.
