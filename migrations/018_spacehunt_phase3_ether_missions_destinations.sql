-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: PHASE 3 - ETHER SHOP, MISSIONS, DESTINATIONS
-- (018_spacehunt_phase3_ether_missions_destinations.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

ALTER TABLE resources ADD COLUMN IF NOT EXISTS ether DOUBLE PRECISION DEFAULT 0.00;

-- Ether trickles in passively (resource.go), scaled by Technology
-- research (0.02 per tick per research level) until the Technology
-- Center building lands in a later phase.
--
-- Commands added: /ether (Ether Shop - convert Ether into Metal, Crystal,
-- Scrap, Neuro Cores, or Cash), /missions (consolidated view of active
-- raids, World Boss engagements, and mining queues), /destinations
-- (previously-scouted rival outposts + the always-available Rogue Drone
-- Nest - matches SpaceHunt's 'map of discovered planets')
