# ADR 0099: Decisions from `internal/bot/handlers/relicconvoy.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/relicconvoy.go` and moved here so the source no longer carries them. Each entry is the original comment.

## RelicConvoyHandler is the panel/claim side of RARE_WORLD_FEATURES_PLAN.md

Removed from `internal/bot/handlers/relicconvoy.go:19`.

~~~~text
RelicConvoyHandler is the panel/claim side of RARE_WORLD_FEATURES_PLAN.md
Phase 1 - the tick.Engine side (spawn/expire) lives in
internal/engine/tick/relicconvoys.go.
~~~~


## HandleRelicPanel (/relic) shows the current live relic convoy, if

Removed from `internal/bot/handlers/relicconvoy.go:30`.

~~~~text
HandleRelicPanel (/relic) shows the current live relic convoy, if
any, with a Claim button, plus a Hall of Relics - the last 10 claimed
relics server-wide, rendered as a Rich Message table (same pattern
exchange.go's listings board and ranking.go's leaderboard use) so a
claimed relic has permanent, visible legacy.
~~~~


## doClaimRelicConvoy is the testable core behind HandleClaimRelicCallback -

Removed from `internal/bot/handlers/relicconvoy.go:97`.

~~~~text
doClaimRelicConvoy is the testable core behind HandleClaimRelicCallback -
race-safe via SELECT ... FOR UPDATE, re-checking claimed_by IS NULL
AND not expired inside the same transaction as the claim, exactly
like doBuyListingQty's listing lock or HandleAttackBossCallback's
boss lock elsewhere in this codebase. On success, credits the
reward (storage-cap clamped) and - only if this is the encampment's
FIRST relic ever - sets a permanent relic_title that is never
overwritten by a later claim.
~~~~


## The relic title is permanent and never overwritten - only the

Removed from `internal/bot/handlers/relicconvoy.go:140`.

~~~~text
The relic title is permanent and never overwritten - only the
FIRST relic an encampment ever wins sets it (see plan doc 1.3).
~~~~


## NEXT_GENERATION_FEATURES_PLAN.md Phase 3: a claimed Relic Convoy

Removed from `internal/bot/handlers/relicconvoy.go:166`.

~~~~text
NEXT_GENERATION_FEATURES_PLAN.md Phase 3: a claimed Relic Convoy
is one of Epic/Legendary gear's only two sources (the other is a
World Boss kill - engine.go's payoutWorldBossLoot). A generous
roll here relative to the boss-kill rate is deliberate - claiming
a relic convoy is already extremely rare server-wide (see
relicconvoys.go's own ~1-per-11-hours spawn cadence), so this is
the "natural second reward tier" the plan doc's own design
describes, not a separate grind.
~~~~
