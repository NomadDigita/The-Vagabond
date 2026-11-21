# ADR 0187: Decisions from `internal/game/gear/gear_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/gear/gear_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestStartCraft_DeductsCostAndQueues is the direct test for

Removed from `internal/game/gear/gear_test.go:59`.

~~~~text
TestStartCraft_DeductsCostAndQueues is the direct test for
NEXT_GENERATION_FEATURES_PLAN.md Phase 3: starting a craft deducts
its CraftCost immediately and queues a crafting_queue row - the item
itself isn't granted yet (see the next test).
~~~~


## 5000 - 500 (scrap_blade's cost)

Removed from `internal/game/gear/gear_test.go:76`.

~~~~text
5000 - 500 (scrap_blade's cost)
~~~~


## TestStartCraft_InsufficientResourcesFails confirms crafting fails

Removed from `internal/game/gear/gear_test.go:95`.

~~~~text
TestStartCraft_InsufficientResourcesFails confirms crafting fails
cleanly (no partial deduction) when the player can't afford it.
~~~~


## scrap_blade (2) + patched_vest (2) = 4; signal_booster (5, but

Removed from `internal/game/gear/gear_test.go:279`.

~~~~text
scrap_blade (2) + patched_vest (2) = 4; signal_booster (5, but
unequipped) must NOT count.
~~~~
