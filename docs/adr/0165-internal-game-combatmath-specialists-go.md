# ADR 0165: Decisions from `internal/game/combatmath/specialists.go`

Status: Accepted

These decisions were written as comments in `internal/game/combatmath/specialists.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package combatmath holds pure, DB-free combat math extracted out of

Removed from `internal/game/combatmath/specialists.go:1`.

~~~~text
Package combatmath holds pure, DB-free combat math extracted out of
internal/engine/tick's raid-resolution loop specifically so it can be
unit-tested directly - the tick engine itself is DB-heavy throughout
and, per this project's conventions, is exercised only via
real-Postgres tests, which makes a narrow arithmetic bug like the one
this file fixes hard to pin down with a fast, deterministic test.
~~~~


## SpecialistToughness is how many "effective casualties" it takes to

Removed from `internal/game/combatmath/specialists.go:26`.

~~~~text
SpecialistToughness is how many "effective casualties" it takes to
destroy one unit of each type - the constants
destroyerToughness/bomberToughness/bcToughness/dsToughness/
liberatorToughness/wraithToughness from
internal/engine/tick/engine.go, unchanged by this fix; only how
they're used to split losses changes here.
~~~~


## Distribute the remainder (totalCasualties - assigned, always a

Removed from `internal/game/combatmath/specialists.go:121`.

~~~~text
Distribute the remainder (totalCasualties - assigned, always a
small non-negative number less than len(slots) from truncation)
to the largest fractional remainders first, skipping any type
already fully depleted.
~~~~


## don't pick the same slot twice in a row for the same remainder unit

Removed from `internal/game/combatmath/specialists.go:142`.

~~~~text
don't pick the same slot twice in a row for the same remainder unit
~~~~


## hazardOf returns a unit type's share weight: more units of a type,

Removed from `internal/game/combatmath/specialists.go:149`.

~~~~text
hazardOf returns a unit type's share weight: more units of a type,
or a lower toughness, both increase how much of the total casualty
pool that type absorbs. A type with zero units or non-positive
toughness (a misconfigured constant) contributes zero hazard rather
than dividing by zero or going negative.
~~~~
