# ADR 0043: Decisions from `internal/bot/handlers/achievements.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/achievements.go` and moved here so the source no longer carries them. Each entry is the original comment.

## AchievementsHandler is the panel side of

Removed from `internal/bot/handlers/achievements.go:13`.

~~~~text
AchievementsHandler is the panel side of
NEXT_GENERATION_FEATURES_PLAN.md Phase 1 - unlocking itself happens
automatically via internal/game/progression.RecordEvent, called from
every real action handler at its own completion point. This handler
only ever reads player_achievements and player_event_counts to
render progress; it never writes to them.
~~~~
