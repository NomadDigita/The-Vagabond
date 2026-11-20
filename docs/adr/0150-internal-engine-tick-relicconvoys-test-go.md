# ADR 0150: Decisions from `internal/engine/tick/relicconvoys_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/relicconvoys_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## runTickPhase runs a single tick-phase function in its own committed

Removed from `internal/engine/tick/relicconvoys_test.go:130`.

~~~~text
runTickPhase runs a single tick-phase function in its own committed
transaction - the non-probabilistic counterpart to
roadencounter_test.go's runUntilCondition, for phases this file's
tests drive deterministically (via the var-not-const roll-chance
override above) rather than by retrying a real random roll.
~~~~
