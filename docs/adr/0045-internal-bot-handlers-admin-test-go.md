# ADR 0045: Decisions from `internal/bot/handlers/admin_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/admin_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestDoSetTaxRate_BroadcastsToAllRealPlayersNotAIFactions verifies the

Removed from `internal/bot/handlers/admin_test.go:8`.

~~~~text
TestDoSetTaxRate_BroadcastsToAllRealPlayersNotAIFactions verifies the
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.3 addition: a tax
rate change now reaches every real player directly, not just the
admin's own confirmation message, while AI factions (which have a
synthetic users row but no Telegram session) are excluded.
~~~~
