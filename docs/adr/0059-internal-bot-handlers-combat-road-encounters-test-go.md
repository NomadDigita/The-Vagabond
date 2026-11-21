# ADR 0059: Decisions from `internal/bot/handlers/combat_road_encounters_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/combat_road_encounters_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestIsRealPlayer_RejectsSyntheticAIFactionIDs guards the fix documented

Removed from `internal/bot/handlers/combat_road_encounters_test.go:5`.

~~~~text
TestIsRealPlayer_RejectsSyntheticAIFactionIDs guards the fix documented
in BUGS_AND_INCONSISTENCIES.md: AI factions' seeded users rows use a
synthetic negative telegram_id, and every "notify the other commander"
site in combat_road_encounters.go must treat that as "nobody to notify,"
not attempt a send that will fail and retry forever.
~~~~


## zero: "no user" sentinel used throughout this codebase

Removed from `internal/bot/handlers/combat_road_encounters_test.go:17`.

~~~~text
zero: "no user" sentinel used throughout this codebase
~~~~
