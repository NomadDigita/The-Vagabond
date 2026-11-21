# ADR 0173: Decisions from `internal/game/devconsole/nlquery_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/nlquery_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Critical safety test: even a maliciously-crafted "intent" value that

Removed from `internal/game/devconsole/nlquery_test.go:129`.

~~~~text
Critical safety test: even a maliciously-crafted "intent" value that
looks like SQL or a destructive instruction must never be treated as
valid — IsKnownIntent must reject it regardless of what
ParseClassification returns, since parsing JSON successfully is not
the same as the intent being safe to run.
~~~~


## See ADR-016: a response cut off mid-object is distinguished from one

Removed from `internal/game/devconsole/nlquery_test.go:153`.

~~~~text
See ADR-016: a response cut off mid-object is distinguished from one
that never contained JSON at all.
~~~~
