# ADR 0118: Decisions from `internal/dbdriver/retrying_test.go`

Status: Accepted

These decisions were written as comments in `internal/dbdriver/retrying_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## fakeConn is a minimal driver.Conn + QueryerContext/ExecerContext

Removed from `internal/dbdriver/retrying_test.go:12`.

~~~~text
fakeConn is a minimal driver.Conn + QueryerContext/ExecerContext
stub standing in for lib/pq's real *pq.conn, so these tests don't
need a live Postgres/PgBouncer to prove the translation logic.
~~~~


## A genuinely different Postgres error (e.g. a unique constraint

Removed from `internal/dbdriver/retrying_test.go:66`.

~~~~text
A genuinely different Postgres error (e.g. a unique constraint
violation) must reach the caller unchanged — retrying it would just
reproduce the same failure and, worse, would mask a real bug behind
a silent retry.
~~~~


## A non-pq error (e.g. a context deadline) is not a pooler artifact

Removed from `internal/dbdriver/retrying_test.go:84`.

~~~~text
A non-pq error (e.g. a context deadline) is not a pooler artifact
either and must also pass through untouched.
~~~~
