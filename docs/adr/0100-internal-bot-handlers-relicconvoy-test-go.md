# ADR 0100: Decisions from `internal/bot/handlers/relicconvoy_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/relicconvoy_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## raiseStorageCap gives a test camp enough storage headroom

Removed from `internal/bot/handlers/relicconvoy_test.go:10`.

~~~~text
raiseStorageCap gives a test camp enough storage headroom
(extension_lvl 40 -> a 40,500 cap, see storagecap.Cap's formula) for
tests that credit reward amounts larger than a fresh level-1 camp's
default 500 cap would allow through un-clamped.
~~~~


## relicCoordCounter guarantees a unique (x, y) per seedRelicConvoy call

Removed from `internal/bot/handlers/relicconvoy_test.go:21`.

~~~~text
relicCoordCounter guarantees a unique (x, y) per seedRelicConvoy call
within a test run - coordinates.(x, y) is UNIQUE, and deriving x from
len(name) (an earlier draft of this helper) risked collisions between
same-length relic names.
~~~~


## TestDoClaimRelicConvoy_FirstClaimSetsTitleAndPaysReward is the direct

Removed from `internal/bot/handlers/relicconvoy_test.go:48`.

~~~~text
TestDoClaimRelicConvoy_FirstClaimSetsTitleAndPaysReward is the direct
test for RARE_WORLD_FEATURES_PLAN.md Phase 1.3: claiming a relic pays
the reward (storage-cap clamped like every other resource-gain path)
and, since this is the encampment's first relic, sets a permanent
relic_title.
~~~~


## 15000 (the relic's own reward) + 3000 (the "relic_hunter"

Removed from `internal/bot/handlers/relicconvoy_test.go:71`.

~~~~text
15000 (the relic's own reward) + 3000 (the "relic_hunter"
achievement, NEXT_GENERATION_FEATURES_PLAN.md Phase 1, unlocked
by this same claim via progression.RecordEvent's
TriggerRelicClaimed hook in doClaimRelicConvoy).
~~~~


## TestDoClaimRelicConvoy_SecondClaimNeverOverwritesTitle covers the

Removed from `internal/bot/handlers/relicconvoy_test.go:96`.

~~~~text
TestDoClaimRelicConvoy_SecondClaimNeverOverwritesTitle covers the
plan doc's explicit "first relic keeps the title" rule: an
encampment that already has a relic_title from an earlier claim must
keep that exact title after claiming a second, different relic.
~~~~


## Both claims should still have paid out though - the title rule

Removed from `internal/bot/handlers/relicconvoy_test.go:130`.

~~~~text
Both claims should still have paid out though - the title rule
only protects the title field, not the reward. 12000 + 18000 =
30000 in relic rewards, plus a one-time 3000 "relic_hunter"
achievement bonus (NEXT_GENERATION_FEATURES_PLAN.md Phase 1)
unlocked on the FIRST claim only (threshold 1, never re-fires on
the second claim) = 33000.
~~~~


## TestDoClaimRelicConvoy_AlreadyClaimedFails covers the race-safety

Removed from `internal/bot/handlers/relicconvoy_test.go:145`.

~~~~text
TestDoClaimRelicConvoy_AlreadyClaimedFails covers the race-safety
requirement: a second claim attempt against an already-claimed
convoy must fail cleanly rather than double-pay.
~~~~
