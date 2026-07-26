-- ==============================================================================
-- THE VAGABOND - MMO LIVING WORLD: PERSISTENT AI CIVILIZATIONS
-- (032_mmo_ai_civilizations.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- Implements MMO_WORLD_EVOLUTION_PLAN.md Phase 6 (partial - see that
-- document's Phase 6 completed-implementation-detail section for exactly
-- what "AI civilization" means in this pass: persistent, discoverable,
-- raidable, passively-growing bases, NOT mobile AI-controlled expeditions).
--
-- AI factions are deliberately modeled as REAL rows in the existing
-- `encampments` table (with a synthetic negative-telegram_id `users` row
-- to satisfy that table's existing NOT NULL/UNIQUE FK on user_id) rather
-- than a new, special-cased entity type. This means the entire existing
-- discovery (`resolveExplorationDiscovery` in internal/engine/tick/
-- engine.go already just queries "any encampment in this continent" with
-- no filter that would exclude one), targeting (`HandleRaidBoard` in
-- internal/bot/handlers/combat.go), raiding, and looting pipeline works on
-- an AI faction for free, with zero special-casing required anywhere in
-- that pipeline.
-- ==============================================================================

ALTER TABLE encampments ADD COLUMN IF NOT EXISTS is_ai_faction BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS ai_faction_key VARCHAR(50);
CREATE UNIQUE INDEX IF NOT EXISTS uq_encampments_ai_faction_key ON encampments(ai_faction_key) WHERE ai_faction_key IS NOT NULL;
