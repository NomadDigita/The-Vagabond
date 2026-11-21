
ALTER TABLE notification_preferences ADD COLUMN IF NOT EXISTS mute_scout_status BOOLEAN NOT NULL DEFAULT FALSE;
