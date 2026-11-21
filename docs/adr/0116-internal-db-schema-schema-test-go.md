# ADR 0116: Decisions from `internal/db/schema/schema_test.go`

Status: Accepted

These decisions were written as comments in `internal/db/schema/schema_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## testDB opens a connection to a real Postgres instance for integration

Removed from `internal/db/schema/schema_test.go:11`.

~~~~text
testDB opens a connection to a real Postgres instance for integration
testing. Phase 7 milestone 5 explicitly asks for tests covering
"migration compatibility, critical idempotency paths" - that can only be
verified against a real database engine, since IF NOT EXISTS / ADD
COLUMN IF NOT EXISTS semantics and constraint-name collisions are
Postgres-specific behavior no pure-Go mock can stand in for.

Skips (not fails) when SCHEMA_TEST_DATABASE_URL isn't set, so this
doesn't break `go test ./...` in environments without a database
available (e.g. CI without a Postgres service, or a quick local run).
~~~~


## TestStatementsNonEmpty is a cheap sanity check that doesn't need a

Removed from `internal/db/schema/schema_test.go:38`.

~~~~text
TestStatementsNonEmpty is a cheap sanity check that doesn't need a
database - if this ever returns an empty slice, every other test in this
package would pass vacuously and hide a real regression.
~~~~


## TestStatementsRunRepeatedlyCleanly is the actual idempotency test: every

Removed from `internal/db/schema/schema_test.go:53`.

~~~~text
TestStatementsRunRepeatedlyCleanly is the actual idempotency test: every
statement must succeed on a bare database (pass 1, matching a fresh
deploy), again against a database that already has the full schema
applied (pass 2, matching every subsequent boot), and a third time for
good measure - guarding against subtler non-idempotency that only shows
up once a constraint-guard row or sequence side-effect from a prior pass
is already present, not just "CREATE TABLE IF NOT EXISTS ran twice".
~~~~
