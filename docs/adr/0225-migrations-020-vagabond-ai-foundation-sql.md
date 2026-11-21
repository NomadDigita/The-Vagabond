# ADR 0225: Decisions from `migrations/020_vagabond_ai_foundation.sql`

Status: Accepted

These decisions were written as comments in `migrations/020_vagabond_ai_foundation.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## 020_vagabond_ai_foundation.sql

Removed from `migrations/020_vagabond_ai_foundation.sql:1`.

~~~~text
020_vagabond_ai_foundation.sql

Phase A — AI Foundation (Vagabond AI Roadmap, independent branch).

This migration is additive-only and touches no table owned by the
parallel SpaceHunt roadmap (phases 1-6). It creates the persistence
layer for internal/ai: cost accounting, feature flags, per-user
permissions, and conversational memory. Every table is namespaced
with an ai_ prefix to make ownership unambiguous during merges.
~~~~


## Seed every known Phase B-J feature flag as enabled so admins have a

Removed from `migrations/020_vagabond_ai_foundation.sql:53`.

~~~~text
Seed every known Phase B-J feature flag as enabled so admins have a
visible row to toggle from day one instead of relying on the
"missing row = enabled" default.
~~~~
