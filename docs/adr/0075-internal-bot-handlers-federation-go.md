# ADR 0075: Decisions from `internal/bot/handlers/federation.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/federation.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Crystal - founding a Federation is a major, deliberate milestone

Removed from `internal/bot/handlers/federation.go:22`.

~~~~text
Crystal - founding a Federation is a major, deliberate milestone
~~~~


## HandleFoundFederation (/fed_found [name]) - a Clan Leader founds a new

Removed from `internal/bot/handlers/federation.go:143`.

~~~~text
HandleFoundFederation (/fed_found [name]) - a Clan Leader founds a new
Federation, deliberately costly since it's a major milestone.
~~~~


## BUGFIX: HandleFoundFederation already blocks founding while in a

Removed from `internal/bot/handlers/federation.go:225`.

~~~~text
BUGFIX: HandleFoundFederation already blocks founding while in a
Federation, but this had no equivalent check - a Leader could
silently switch Federations without ever running /fed_leave.
~~~~
