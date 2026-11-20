
CREATE TABLE IF NOT EXISTS governor_settings (
    encampment_id      UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
    autopilot_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
