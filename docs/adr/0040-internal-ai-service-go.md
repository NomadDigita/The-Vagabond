# ADR 0040: Decisions from `internal/ai/service.go`

Status: Accepted

These decisions were written as comments in `internal/ai/service.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Never cache a response that got cut off mid-completion —

Removed from `internal/ai/service.go:101`.

~~~~text
Never cache a response that got cut off mid-completion —
doing so would lock every caller (including a player
tapping "Refresh") into replaying the same truncated
output for the rest of the TTL window instead of getting
a fresh, hopefully-complete retry. See ADR-025 in
PROJECT_MASTER_PLAN.md.
~~~~
