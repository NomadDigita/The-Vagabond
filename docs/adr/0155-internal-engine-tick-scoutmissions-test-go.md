# ADR 0155: Decisions from `internal/engine/tick/scoutmissions_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/scoutmissions_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## 50/hour/scout * 2 hours * 5 scouts = 500 total value, 60/40 split.

Removed from `internal/engine/tick/scoutmissions_test.go:204`.

~~~~text
50/hour/scout * 2 hours * 5 scouts = 500 total value, 60/40 split.
~~~~


## Gate now closed (just notified) - a second immediate call must not

Removed from `internal/engine/tick/scoutmissions_test.go:268`.

~~~~text
Gate now closed (just notified) - a second immediate call must not
double-ping.
~~~~
