# ADR 0161: Decisions from `internal/game/battleanalyst/analyst.go`

Status: Accepted

These decisions were written as comments in `internal/game/battleanalyst/analyst.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/battleanalyst/analyst.go:12`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from every other Phase B+ scope.
~~~~


## buildAttackerStats mirrors fleetcommander.BuildCombatHistory's exact

Removed from `internal/game/battleanalyst/analyst.go:69`.

~~~~text
buildAttackerStats mirrors fleetcommander.BuildCombatHistory's exact
query and "apparent win" heuristic (stolen resources > 0), extended
here to also total the stolen value itself. Kept as an independent
query rather than calling into fleetcommander — this package has no
dependency on fleetcommander per the usual per-package isolation
trade-off already documented in researchplanner's doc comment.
~~~~


## buildDefenderStats reads the same raids table from the opposite

Removed from `internal/game/battleanalyst/analyst.go:108`.

~~~~text
buildDefenderStats reads the same raids table from the opposite
side. "Apparent win" for a defender is the inverse heuristic: the
attacker walked away with nothing stolen, meaning the defense held.
~~~~


## Recommend produces a fresh AI analysis of the given player's combat

Removed from `internal/game/battleanalyst/analyst.go:166`.

~~~~text
Recommend produces a fresh AI analysis of the given player's combat
record. It stores both turns in ai_memory under MemoryScope.

Read-only: nothing in this method changes any raid, arena battle, or
unit — it only reads and summarizes what already happened.
~~~~
