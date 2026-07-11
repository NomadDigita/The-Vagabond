-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: REAL RETURN-MARCH TIMING
-- (011_spacehunt_slice9_march_timing.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs the statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

-- Stores the real outbound march duration so the return trip can be
-- anchored to it (previously the return trip used a flat, disconnected
-- 15-minute base regardless of how far the raid actually traveled).
ALTER TABLE raids ADD COLUMN IF NOT EXISTS base_march_minutes DOUBLE PRECISION DEFAULT 15.0;
