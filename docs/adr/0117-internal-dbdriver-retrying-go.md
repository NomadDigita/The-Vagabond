# ADR 0117: Decisions from `internal/dbdriver/retrying.go`

Status: Accepted

These decisions were written as comments in `internal/dbdriver/retrying.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package dbdriver wraps lib/pq's "postgres" database/sql driver to

Removed from `internal/dbdriver/retrying.go:1`.

~~~~text
Package dbdriver wraps lib/pq's "postgres" database/sql driver to
transparently retry queries that fail with Postgres error 26000
(invalid_sql_statement_name — surfaces as "pq: unnamed prepared
statement does not exist").

Root cause (see ADR-026, PROJECT_MASTER_PLAN.md): Supabase's
connection pooler runs PgBouncer in transaction pooling mode. lib/pq
executes every parameterized query via the Postgres extended query
protocol using an unnamed prepared statement (Parse, then Bind +
Execute in a later step). PgBouncer's transaction pooling can hand
the Bind/Execute half of that exchange to a different backend
server than the one that saw the Parse, so the backend genuinely
has never heard of the statement lib/pq is trying to run. It's a
pooler artifact, not a real query error, and it's intermittent by
nature — which is exactly the "sometimes it brings out this error"
behavior reported against the AI Developer Console's Balance and
Weekly reports (they run several queries back-to-back, so they're
where a player is statistically most likely to notice one land
wrong).

The fix doesn't touch any of the ~dozens of call sites across the
codebase that use co.DB.QueryRowContext/QueryContext/ExecContext
directly: this driver sits underneath database/sql itself. When it
sees error code 26000, it returns driver.ErrBadConn instead of the
real error. database/sql already has a built-in, well-tested
mechanism for exactly this signal: on driver.ErrBadConn from a
query that hasn't started returning rows yet, it discards the
connection and transparently retries once on a fresh one — see
maxBadConnRetries in the standard library's database/sql/sql.go.
That fresh connection gets its own Parse, so the retry succeeds
essentially every time.
~~~~


## pgInvalidSQLStatementName is the Postgres SQLSTATE for "unnamed

Removed from `internal/dbdriver/retrying.go:42`.

~~~~text
pgInvalidSQLStatementName is the Postgres SQLSTATE for "unnamed
prepared statement does not exist" — see
https://www.postgresql.org/docs/current/errcodes-appendix.html,
class 26 (Invalid SQL Statement Name).
~~~~


## Register installs a database/sql driver under the given name that

Removed from `internal/dbdriver/retrying.go:48`.

~~~~text
Register installs a database/sql driver under the given name that
wraps lib/pq's "postgres" driver with the retry behavior described
in the package doc. Call this once during startup, before
sql.Open(name, dsn) — see cmd/bot/main.go.
~~~~


## isPoolerArtifact reports whether err is the specific, known-benign

Removed from `internal/dbdriver/retrying.go:56`.

~~~~text
isPoolerArtifact reports whether err is the specific, known-benign
"unnamed prepared statement does not exist" error caused by a
PgBouncer transaction-mode connection handoff, as opposed to a real
query problem (bad SQL, constraint violation, etc.) that a retry
would just reproduce identically.
~~~~


## retryingConn forwards every driver.Conn call straight through to

Removed from `internal/dbdriver/retrying.go:76`.

~~~~text
retryingConn forwards every driver.Conn call straight through to
the underlying lib/pq connection, except that QueryContext/
ExecContext (and their pre-context Query/Exec counterparts, kept
for interface completeness) translate isPoolerArtifact errors into
driver.ErrBadConn so database/sql retries on a new connection
instead of surfacing the pooler artifact to the caller.
~~~~


## Begin satisfies the (deprecated but still required) driver.Conn

Removed from `internal/dbdriver/retrying.go:94`.

~~~~text
Begin satisfies the (deprecated but still required) driver.Conn
interface; database/sql always prefers BeginTx below when present.
~~~~


## Query and Exec (the pre-context driver.Queryer/driver.Execer

Removed from `internal/dbdriver/retrying.go:138`.

~~~~text
Query and Exec (the pre-context driver.Queryer/driver.Execer
interfaces) are implemented for completeness and the same
pooler-artifact translation, though modern database/sql always
prefers the *Context variants above when a driver implements them,
as lib/pq's connection does.
~~~~


## CheckNamedValue forwards to lib/pq's own value conversion so

Removed from `internal/dbdriver/retrying.go:188`.

~~~~text
CheckNamedValue forwards to lib/pq's own value conversion so
driver-specific types (e.g. pq.Array, pq.StringArray) keep working
exactly as they would through the unwrapped "postgres" driver.
~~~~
