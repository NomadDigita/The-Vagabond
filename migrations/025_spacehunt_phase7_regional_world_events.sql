
ALTER TABLE world_events ADD COLUMN IF NOT EXISTS continent VARCHAR(50) NOT NULL DEFAULT 'Global';
CREATE INDEX IF NOT EXISTS idx_world_events_continent ON world_events(continent, expires_at);

