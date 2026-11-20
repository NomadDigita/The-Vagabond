# ADR 0210: Decisions from `internal/game/roadcombat/roadcombat_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/roadcombat/roadcombat_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Returning at the same 0.5 progress should be symmetric (also midpoint),

Removed from `internal/game/roadcombat/roadcombat_test.go:35`.

~~~~text
Returning at the same 0.5 progress should be symmetric (also midpoint),
but returning progress=0 should sit AT the destination (just turned
around) and progress=1 should be back at origin.
~~~~
