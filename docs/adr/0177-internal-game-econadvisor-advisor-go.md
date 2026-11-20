# ADR 0177: Decisions from `internal/game/econadvisor/advisor.go`

Status: Accepted

These decisions were written as comments in `internal/game/econadvisor/advisor.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/econadvisor/advisor.go:12`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from every other Phase B+ scope.
~~~~


## resourceColumns lists every resources-table column this package

Removed from `internal/game/econadvisor/advisor.go:20`.

~~~~text
resourceColumns lists every resources-table column this package
knows about, explicitly, so a schema change is a deliberate one-line
addition rather than a silent behavior change (same pattern as
fleetcommander.unitColumns).
~~~~


## bank_accounts may not have a row yet for a brand-new player —

Removed from `internal/game/econadvisor/advisor.go:79`.

~~~~text
bank_accounts may not have a row yet for a brand-new player —
that's a valid all-zero state, not an error.
~~~~


## Recommend produces a fresh AI recommendation for the given player's

Removed from `internal/game/econadvisor/advisor.go:141`.

~~~~text
Recommend produces a fresh AI recommendation for the given player's
economy. It stores both turns in ai_memory under MemoryScope.

Advisory-only: nothing in this method buys, sells, upgrades, or
otherwise mutates any game table.
~~~~
