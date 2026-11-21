# ADR 0145: Decisions from `internal/engine/tick/economiccollapse_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/economiccollapse_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestEconomicCollapseWarning verifies AI_PARITY_AND_WORLD_NOTIFICATIONS_

Removed from `internal/engine/tick/economiccollapse_test.go:8`.

~~~~text
TestEconomicCollapseWarning verifies AI_PARITY_AND_WORLD_NOTIFICATIONS_
PLAN.md section 1.5's "communicate this is possible" addition: a
defeated defender left with a critically low resource total gets an
explicit warning appended to their battle report, while one still
holding a meaningful amount doesn't.
~~~~
