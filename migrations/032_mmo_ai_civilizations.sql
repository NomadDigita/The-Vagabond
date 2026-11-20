
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS is_ai_faction BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS ai_faction_key VARCHAR(50);
CREATE UNIQUE INDEX IF NOT EXISTS uq_encampments_ai_faction_key ON encampments(ai_faction_key) WHERE ai_faction_key IS NOT NULL;
