# ADR 0127: Decisions from `internal/engine/realtime/listener.go`

Status: Accepted

These decisions were written as comments in `internal/engine/realtime/listener.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Same defense-in-depth as the tick engine and

Removed from `internal/engine/realtime/listener.go:57`.

~~~~text
Same defense-in-depth as the tick engine and
notification dispatcher: an unrecovered panic in
dispatchNotification would otherwise kill this
goroutine and silently stop all future realtime
(LISTEN/NOTIFY-driven) push events forever.
~~~~


## Defensive UUID length validation to prevent PostgreSQL "invalid input syntax for type uuid

Removed from `internal/engine/realtime/listener.go:86`.

~~~~text
Defensive UUID length validation to prevent PostgreSQL "invalid input syntax for type uuid" aborts
~~~~


## Query message contents and mark as sent in a single transaction with FOR UPDATE row locks

Removed from `internal/engine/realtime/listener.go:94`.

~~~~text
Query message contents and mark as sent in a single transaction with FOR UPDATE row locks
~~~~


## Handled successfully by backup polling dispatcher

Removed from `internal/engine/realtime/listener.go:118`.

~~~~text
Handled successfully by backup polling dispatcher
~~~~
