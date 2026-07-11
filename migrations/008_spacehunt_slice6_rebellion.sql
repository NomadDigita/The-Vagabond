-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: THE REBELLION
-- (008_spacehunt_slice6_rebellion.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

CREATE TABLE IF NOT EXISTS rebellion_support (
	encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
	total_contributed DOUBLE PRECISION DEFAULT 0
);
