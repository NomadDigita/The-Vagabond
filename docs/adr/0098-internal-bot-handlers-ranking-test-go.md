# ADR 0098: Decisions from `internal/bot/handlers/ranking_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/ranking_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestDisplayName_PrefixesAIFactions verifies the leaderboard's honest-

Removed from `internal/bot/handlers/ranking_test.go:14`.

~~~~text
TestDisplayName_PrefixesAIFactions verifies the leaderboard's honest-
labeling rule from AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section
1.2: AI factions get a 🤖 prefix, humans don't, and both are HTML-escaped.
~~~~


## world_bosses (NEXT_GENERATION_FEATURES_PLAN.md Phase 8) has no FK

Removed from `internal/bot/handlers/ranking_test.go:48`.

~~~~text
world_bosses (NEXT_GENERATION_FEATURES_PLAN.md Phase 8) has no FK
to encampments/users/coordinates, so it survives the CASCADE
truncate above untouched - reset it explicitly so tier-gating
tests (boss_test.go) start from a clean slate instead of
accumulating total_kills across the whole shared-DB test binary
run. See roadencounter_test.go's testDB for the tick-package
counterpart of this same fix.
~~~~


## TestTopPlayersQuery_IncludesAIFactionsRankedByScore proves the leaderboard

Removed from `internal/bot/handlers/ranking_test.go:84`.

~~~~text
TestTopPlayersQuery_IncludesAIFactionsRankedByScore proves the leaderboard
fix: an AI faction with a higher score than a human now outranks that
human in the "top players" query, instead of being filtered out.
~~~~
