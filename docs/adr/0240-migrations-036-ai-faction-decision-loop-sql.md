# ADR 0240: Decisions from `migrations/036_ai_faction_decision_loop.sql`

Status: Accepted

These decisions were written as comments in `migrations/036_ai_faction_decision_loop.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## ==============================================================================

Removed from `migrations/036_ai_faction_decision_loop.sql:1`.

~~~~text
==============================================================================
THE VAGABOND - AI FACTION DECISION LOOP (036_ai_faction_decision_loop.sql)
DB Engine: PostgreSQL (Supabase)

Second half of Phase 6 (persistent AI civilizations). See
AI_FACTION_DECISION_LOOP_PLAN.md for the full design - this migration
only adds the cadence tracking and decision audit trail; it does not
change how an AI faction is represented (still a real `encampments` row
with is_ai_faction = TRUE, unchanged from the foundational tier).
==============================================================================
~~~~


## Every AI decision, whether or not it resulted in a raid - this is what

Removed from `migrations/036_ai_faction_decision_loop.sql:14`.

~~~~text
Every AI decision, whether or not it resulted in a raid - this is what
makes AI behavior debuggable instead of a black box, and is the source
for a future devconsole "ai_activity" metric (see
AI_FACTION_DECISION_LOOP_PLAN.md).
~~~~
