# ADR 0226: Decisions from `migrations/021_vagabond_ai_governor.sql`

Status: Accepted

These decisions were written as comments in `migrations/021_vagabond_ai_governor.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## 021_vagabond_ai_governor.sql

Removed from `migrations/021_vagabond_ai_governor.sql:1`.

~~~~text
021_vagabond_ai_governor.sql

Phase B — AI Planet Governor (Vagabond AI Roadmap).

Stores each player's autopilot preference. As of this migration,
nothing reads autopilot_enabled to take autonomous action — see
PROJECT_MASTER_PLAN.md §4 (Known Issues / Technical Debt) for why
that execution engine is deliberately not built yet. The column
exists now so the data model and player-facing toggle are already
in place when it is.
~~~~
