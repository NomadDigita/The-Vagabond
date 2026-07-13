-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 2 - GUILD EXTRAS
-- (017_spacehunt_phase2_guild_extras.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

ALTER TABLE clans ADD COLUMN IF NOT EXISTS icon VARCHAR(10) DEFAULT '🏴';
ALTER TABLE clans ADD COLUMN IF NOT EXISTS description TEXT DEFAULT '';
ALTER TABLE clans ADD COLUMN IF NOT EXISTS recruiting BOOLEAN DEFAULT TRUE;

-- Commands added: /guild_missions (recent raids/transfers involving your
-- Clan), /guildmsg [message] (broadcast to all members), /guild_icon
-- (random animal icon, Leader-only), /guild_description [text]
-- (Leader-only), /board (recruitment post board of clans actively
-- recruiting, distinct from /clans which is the pure ranking list)
