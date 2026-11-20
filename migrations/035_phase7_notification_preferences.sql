
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS category VARCHAR(30) NOT NULL DEFAULT 'general';
CREATE INDEX IF NOT EXISTS idx_notifications_pending_category ON notifications(category) WHERE is_sent = FALSE;

CREATE TABLE IF NOT EXISTS notification_preferences (
	user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
	mute_route_status BOOLEAN NOT NULL DEFAULT FALSE,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS speedup_usage_log (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	scrap_spent DOUBLE PRECISION NOT NULL,
	dollars_spent DOUBLE PRECISION NOT NULL,
	crystal_spent DOUBLE PRECISION NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_speedup_usage_log_created ON speedup_usage_log(created_at);

