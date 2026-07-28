-- ==============================================================================
-- THE VAGABOND - AI FACTION DECISION LOOP (036_ai_faction_decision_loop.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- Second half of Phase 6 (persistent AI civilizations). See
-- AI_FACTION_DECISION_LOOP_PLAN.md for the full design - this migration
-- only adds the cadence tracking and decision audit trail; it does not
-- change how an AI faction is represented (still a real `encampments` row
-- with is_ai_faction = TRUE, unchanged from the foundational tier).
-- ==============================================================================

ALTER TABLE encampments ADD COLUMN IF NOT EXISTS ai_last_decision_at TIMESTAMP WITH TIME ZONE;

-- Every AI decision, whether or not it resulted in a raid - this is what
-- makes AI behavior debuggable instead of a black box, and is the source
-- for a future devconsole "ai_activity" metric (see
-- AI_FACTION_DECISION_LOOP_PLAN.md).
CREATE TABLE IF NOT EXISTS ai_faction_decisions (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	decided_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
	intent VARCHAR(20) NOT NULL,
	target_encampment_id UUID REFERENCES encampments(id) ON DELETE SET NULL,
	resulting_raid_id UUID REFERENCES raids(id) ON DELETE SET NULL,
	reason TEXT,
	CONSTRAINT ai_faction_decisions_intent CHECK (intent IN ('scout', 'raid', 'idle'))
);
CREATE INDEX IF NOT EXISTS idx_ai_faction_decisions_encampment ON ai_faction_decisions(encampment_id, decided_at DESC);
