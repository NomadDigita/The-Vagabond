# ADR 0195: Decisions from `internal/game/herodepth/herodepth_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/herodepth/herodepth_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestTrainingCostMultiplier_StacksDiscountsWithFloor confirms

Removed from `internal/game/herodepth/herodepth_test.go:111`.

~~~~text
TestTrainingCostMultiplier_StacksDiscountsWithFloor confirms
unlocked EffectTrainingCostPct nodes stack multiplicatively-additive
(summed as percentages) and never drop the multiplier below the 20%
floor even with many stacked discounts.
~~~~
