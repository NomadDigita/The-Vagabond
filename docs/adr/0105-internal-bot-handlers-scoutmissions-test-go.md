# ADR 0105: Decisions from `internal/bot/handlers/scoutmissions_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/scoutmissions_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Put the first-dispatched mission into 'returning' so the two phases

Removed from `internal/bot/handlers/scoutmissions_test.go:187`.

~~~~text
Put the first-dispatched mission into 'returning' so the two phases
are both represented, and confirm both survive independently in the
listing (i.e. neither overwrites the other, which an ORDER BY...
LIMIT 1 query would have done before this feature).
~~~~
