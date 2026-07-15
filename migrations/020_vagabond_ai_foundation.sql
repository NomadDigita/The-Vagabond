-- 020_vagabond_ai_foundation.sql
--
-- Phase A — AI Foundation (Vagabond AI Roadmap, independent branch).
--
-- This migration is additive-only and touches no table owned by the
-- parallel SpaceHunt roadmap (phases 1-6). It creates the persistence
-- layer for internal/ai: cost accounting, feature flags, per-user
-- permissions, and conversational memory. Every table is namespaced
-- with an ai_ prefix to make ownership unambiguous during merges.

CREATE TABLE IF NOT EXISTS ai_feature_flags (
    feature     VARCHAR(50) PRIMARY KEY,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ai_permissions (
    user_id     BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
    feature     VARCHAR(50) NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, feature)
);

CREATE TABLE IF NOT EXISTS ai_memory (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
    scope        VARCHAR(100) NOT NULL,
    role         VARCHAR(20) NOT NULL,
    content      TEXT NOT NULL,
    tool_call_id VARCHAR(100),
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ai_memory_user_scope_time
    ON ai_memory (user_id, scope, created_at);

CREATE TABLE IF NOT EXISTS ai_cost_log (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        BIGINT REFERENCES users(telegram_id) ON DELETE SET NULL,
    feature        VARCHAR(50) NOT NULL,
    provider       VARCHAR(50) NOT NULL,
    model          VARCHAR(100) NOT NULL,
    input_tokens   INT NOT NULL DEFAULT 0,
    output_tokens  INT NOT NULL DEFAULT 0,
    cost_usd       DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ai_cost_log_user_time ON ai_cost_log (user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_ai_cost_log_time ON ai_cost_log (created_at);

-- Seed every known Phase B-J feature flag as enabled so admins have a
-- visible row to toggle from day one instead of relying on the
-- "missing row = enabled" default.
INSERT INTO ai_feature_flags (feature, enabled) VALUES
    ('ai_planet_governor', TRUE),
    ('ai_fleet_commander', TRUE),
    ('ai_economy_advisor', TRUE),
    ('ai_research_planner', TRUE),
    ('ai_battle_analyst', TRUE),
    ('ai_guild_assistant', TRUE),
    ('ai_dynamic_galaxy', TRUE),
    ('ai_npc_intelligence', TRUE),
    ('ai_developer_console', TRUE)
ON CONFLICT (feature) DO NOTHING;
