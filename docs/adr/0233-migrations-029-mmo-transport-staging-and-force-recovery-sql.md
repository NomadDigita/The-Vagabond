# ADR 0233: Decisions from `migrations/029_mmo_transport_staging_and_force_recovery.sql`

Status: Accepted

These decisions were written as comments in `migrations/029_mmo_transport_staging_and_force_recovery.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## ==============================================================================

Removed from `migrations/029_mmo_transport_staging_and_force_recovery.sql:1`.

~~~~text
==============================================================================
THE VAGABOND - MMO LOGISTICS HARDENING: STAGED TRANSPORTS + FORCE RECOVERY
(029_mmo_transport_staging_and_force_recovery.sql)
DB Engine: PostgreSQL (Supabase)

Transport vehicles must be in a campaign column, not merely owned at home.
Matching idempotent statements are in cmd/bot/main.go.
==============================================================================
~~~~
