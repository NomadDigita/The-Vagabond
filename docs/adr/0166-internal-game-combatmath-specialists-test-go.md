# ADR 0166: Decisions from `internal/game/combatmath/specialists_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/combatmath/specialists_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## realToughness mirrors the constants in

Removed from `internal/game/combatmath/specialists_test.go:9`.

~~~~text
realToughness mirrors the constants in
internal/engine/tick/engine.go's resolveRaidCombats
(destroyerToughness, bomberToughness, bcToughness, dsToughness,
liberatorToughness, wraithToughness) as of the 2026-08-05 fix.
~~~~


## TestAllocateSpecialistLosses_ReproducesReportedScenario is a direct

Removed from `internal/game/combatmath/specialists_test.go:22`.

~~~~text
TestAllocateSpecialistLosses_ReproducesReportedScenario is a direct
regression test for the live bug report (2026-08-05): a fleet of
Destroyers 14 / Bombers 5 / Liberators 34 / Wraiths 32 /
Battlecruisers 30 / Doomsday Rigs 4 lost all 4 Doomsday Rigs within
2-3 combat rounds under the old headcount-only split. Under the new
hazard-weighted split, a Doomsday Rig - 50x tougher than a Destroyer
and 10x tougher than a Battlecruiser - must come out receiving a
share of losses far below its ~3.4% headcount share of the pool
(4 of 119 units).
~~~~


## Run a large total so the Rig's expected share (a fraction of a

Removed from `internal/game/combatmath/specialists_test.go:41`.

~~~~text
Run a large total so the Rig's expected share (a fraction of a
single unit under the old per-round math) becomes large enough
to compare cleanly - equivalent to simulating many rounds' worth
of specialist-pool casualties at once.
~~~~


## With these numbers the Rig's hazard share is roughly 30x smaller

Removed from `internal/game/combatmath/specialists_test.go:53`.

~~~~text
With these numbers the Rig's hazard share is roughly 30x smaller
than its headcount share (4/200=0.02 vs 4/119=0.034, against a
pool whose total hazard is dominated by the many low-toughness
Wraiths/Destroyers) - assert it lands in that ballpark rather
than pinning an exact figure that would break on unrelated
toughness-constant tuning.
~~~~
