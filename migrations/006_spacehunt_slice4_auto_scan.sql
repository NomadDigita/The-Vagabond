-- ==============================================================================
-- THE VAGABOND — SPACEHUNT REVIVAL: AUTOMATIC SCAN
-- (006_spacehunt_slice4_auto_scan.sql)
-- DB Engine: PostgreSQL (Supabase)
--
-- NOTE: main.go already runs every statement below automatically on every
-- bot startup. This file is a readable reference only.
-- ==============================================================================

ALTER TABLE encampments ADD COLUMN IF NOT EXISTS auto_scan_enabled BOOLEAN DEFAULT FALSE;
