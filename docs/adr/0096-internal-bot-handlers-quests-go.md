# ADR 0096: Decisions from `internal/bot/handlers/quests.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/quests.go` and moved here so the source no longer carries them. Each entry is the original comment.

## QuestsHandler is the panel/claim side of

Removed from `internal/bot/handlers/quests.go:14`.

~~~~text
QuestsHandler is the panel/claim side of
NEXT_GENERATION_FEATURES_PLAN.md Phase 2 - progress itself is
tracked automatically via internal/game/progression.RecordEvent,
called from every real action handler at its own completion point.
This handler renders the board and handles the Claim button; all
the actual claim logic lives in progression.ClaimQuest so both the
panel and a future non-Telegram surface (if one's ever added) share
identical rules.
~~~~
