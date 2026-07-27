-- ==============================================================================
-- THE VAGABOND - PHASE 7 MILESTONE 2: NOTIFICATION PREFERENCES + DEDUPLICATION
-- (035_phase7_notification_preferences.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- MMO_WORLD_EVOLUTION_PLAN.md Phase 7 milestone 2: "Add notification
-- preferences/deduplication where high-volume events could overwhelm
-- players, without suppressing combat, discovery, or supply-loss alerts."
--
-- Two independent mechanisms, deliberately scoped:
--
-- 1. category on notifications + notification_preferences: lets a player
--    opt out of exactly one high-volume, low-stakes category
--    ('route_status' - peaceful road-encounter passes, weather clears,
--    convoy arrivals). Combat, discovery, and supply-loss notifications
--    are never tagged with a mutable category, so they can never be
--    silenced by this mechanism - see notifications/preferences.go's
--    MutableCategories, which is the single source of truth for what's
--    allowed to be muted.
--
-- 2. Dedup is handled at the dispatcher level (see
--    internal/engine/notifications/notifications.go), not by a new
--    column: identical (user_id, message) pairs still pending in the
--    same drain cycle are sent once and marked sent together. This is a
--    general fix that helps every notification category, not just new
--    ones, without needing every one of this codebase's many INSERT INTO
--    notifications call sites to be individually retrofitted.
-- ==============================================================================

ALTER TABLE notifications ADD COLUMN IF NOT EXISTS category VARCHAR(30) NOT NULL DEFAULT 'general';
CREATE INDEX IF NOT EXISTS idx_notifications_pending_category ON notifications(category) WHERE is_sent = FALSE;

CREATE TABLE IF NOT EXISTS notification_preferences (
	user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
	mute_route_status BOOLEAN NOT NULL DEFAULT FALSE,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Phase 7 milestone 3 wants a "speed-up expenditure" admin metric, but
-- nothing previously logged an emergency speed-up as a structured event
-- (event_log is a free-text activity feed, not aggregable). This is a
-- forward-looking log only - it has no historical data before this
-- migration, which the devconsole speedup_expenditure intent says
-- explicitly rather than implying a false zero-usage baseline.
CREATE TABLE IF NOT EXISTS speedup_usage_log (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
	scrap_spent DOUBLE PRECISION NOT NULL,
	dollars_spent DOUBLE PRECISION NOT NULL,
	crystal_spent DOUBLE PRECISION NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_speedup_usage_log_created ON speedup_usage_log(created_at);

