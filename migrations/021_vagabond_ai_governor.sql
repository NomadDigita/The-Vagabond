-- 021_vagabond_ai_governor.sql
--
-- Phase B — AI Planet Governor (Vagabond AI Roadmap).
--
-- Stores each player's autopilot preference. As of this migration,
-- nothing reads autopilot_enabled to take autonomous action — see
-- PROJECT_MASTER_PLAN.md §4 (Known Issues / Technical Debt) for why
-- that execution engine is deliberately not built yet. The column
-- exists now so the data model and player-facing toggle are already
-- in place when it is.

CREATE TABLE IF NOT EXISTS governor_settings (
    encampment_id      UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
    autopilot_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
