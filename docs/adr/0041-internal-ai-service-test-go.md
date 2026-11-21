# ADR 0041: Decisions from `internal/ai/service_test.go`

Status: Accepted

These decisions were written as comments in `internal/ai/service_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## A truncated response must never be cached: caching it would lock a

Removed from `internal/ai/service_test.go:168`.

~~~~text
A truncated response must never be cached: caching it would lock a
player who taps "Refresh" (or anything else that re-issues the same
request) into replaying the same cut-off text for the rest of the
TTL window instead of getting a fresh, hopefully-complete retry.
~~~~
