# ADR 0081: Decisions from `internal/bot/handlers/ghostprotocol_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/ghostprotocol_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestDoGhostProtocol_DeductsProportionalCostAndClearsLocks verifies the

Removed from `internal/bot/handlers/ghostprotocol_test.go:9`.

~~~~text
TestDoGhostProtocol_DeductsProportionalCostAndClearsLocks verifies the
core mechanic from AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section
3.4: relocates the camp, deducts ghostProtocolCostFraction of current
resources (not a flat number), and deletes every known_locations row
targeting this camp while leaving encampment_discoveries untouched.
~~~~


## TestDoGhostProtocol_RespectsExistingCooldown covers the cooldown gate

Removed from `internal/bot/handlers/ghostprotocol_test.go:99`.

~~~~text
TestDoGhostProtocol_RespectsExistingCooldown covers the cooldown gate
directly, independent of having just executed once.
~~~~
