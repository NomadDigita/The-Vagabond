
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS ai_last_decision_at TIMESTAMP WITH TIME ZONE;

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
