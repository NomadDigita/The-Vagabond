-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 1 - IDENTITY & SOCIAL COMMANDS
-- (016_spacehunt_phase1_identity_social.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS notify_on_raid BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS notify_on_storage_full BOOLEAN DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS referred_by BIGINT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code VARCHAR(20);

CREATE TABLE IF NOT EXISTS user_mutes (
	muter_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	muted_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (muter_id, muted_id)
);

CREATE TABLE IF NOT EXISTS event_log (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	message TEXT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS feedback_submissions (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
	message TEXT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Commands added: /description, /settings, /refer, /feedback, /msg,
-- /mute, /unmute, /mutes, /log, /stats, /units
