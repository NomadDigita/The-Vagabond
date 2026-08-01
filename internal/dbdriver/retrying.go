// Package dbdriver wraps lib/pq's "postgres" database/sql driver to
// transparently retry queries that fail with Postgres error 26000
// (invalid_sql_statement_name — surfaces as "pq: unnamed prepared
// statement does not exist").
//
// Root cause (see ADR-026, PROJECT_MASTER_PLAN.md): Supabase's
// connection pooler runs PgBouncer in transaction pooling mode. lib/pq
// executes every parameterized query via the Postgres extended query
// protocol using an unnamed prepared statement (Parse, then Bind +
// Execute in a later step). PgBouncer's transaction pooling can hand
// the Bind/Execute half of that exchange to a different backend
// server than the one that saw the Parse, so the backend genuinely
// has never heard of the statement lib/pq is trying to run. It's a
// pooler artifact, not a real query error, and it's intermittent by
// nature — which is exactly the "sometimes it brings out this error"
// behavior reported against the AI Developer Console's Balance and
// Weekly reports (they run several queries back-to-back, so they're
// where a player is statistically most likely to notice one land
// wrong).
//
// The fix doesn't touch any of the ~dozens of call sites across the
// codebase that use co.DB.QueryRowContext/QueryContext/ExecContext
// directly: this driver sits underneath database/sql itself. When it
// sees error code 26000, it returns driver.ErrBadConn instead of the
// real error. database/sql already has a built-in, well-tested
// mechanism for exactly this signal: on driver.ErrBadConn from a
// query that hasn't started returning rows yet, it discards the
// connection and transparently retries once on a fresh one — see
// maxBadConnRetries in the standard library's database/sql/sql.go.
// That fresh connection gets its own Parse, so the retry succeeds
// essentially every time.
package dbdriver

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/lib/pq"
)

// pgInvalidSQLStatementName is the Postgres SQLSTATE for "unnamed
// prepared statement does not exist" — see
// https://www.postgresql.org/docs/current/errcodes-appendix.html,
// class 26 (Invalid SQL Statement Name).
const pgInvalidSQLStatementName = "26000"

// Register installs a database/sql driver under the given name that
// wraps lib/pq's "postgres" driver with the retry behavior described
// in the package doc. Call this once during startup, before
// sql.Open(name, dsn) — see cmd/bot/main.go.
func Register(name string) {
	sql.Register(name, retryingDriver{})
}

// isPoolerArtifact reports whether err is the specific, known-benign
// "unnamed prepared statement does not exist" error caused by a
// PgBouncer transaction-mode connection handoff, as opposed to a real
// query problem (bad SQL, constraint violation, etc.) that a retry
// would just reproduce identically.
func isPoolerArtifact(err error) bool {
	pqErr, ok := err.(*pq.Error)
	return ok && pqErr.Code == pgInvalidSQLStatementName
}

type retryingDriver struct{}

func (retryingDriver) Open(dsn string) (driver.Conn, error) {
	conn, err := (pq.Driver{}).Open(dsn)
	if err != nil {
		return nil, err
	}
	return &retryingConn{conn: conn}, nil
}

// retryingConn forwards every driver.Conn call straight through to
// the underlying lib/pq connection, except that QueryContext/
// ExecContext (and their pre-context Query/Exec counterparts, kept
// for interface completeness) translate isPoolerArtifact errors into
// driver.ErrBadConn so database/sql retries on a new connection
// instead of surfacing the pooler artifact to the caller.
type retryingConn struct {
	conn driver.Conn
}

func (c *retryingConn) Prepare(query string) (driver.Stmt, error) {
	return c.conn.Prepare(query)
}

func (c *retryingConn) Close() error {
	return c.conn.Close()
}

// Begin satisfies the (deprecated but still required) driver.Conn
// interface; database/sql always prefers BeginTx below when present.
func (c *retryingConn) Begin() (driver.Tx, error) { //nolint:staticcheck
	return c.conn.Begin() //nolint:staticcheck
}

func (c *retryingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if pc, ok := c.conn.(driver.ConnPrepareContext); ok {
		return pc.PrepareContext(ctx, query)
	}
	return c.conn.Prepare(query)
}

func (c *retryingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if bt, ok := c.conn.(driver.ConnBeginTx); ok {
		return bt.BeginTx(ctx, opts)
	}
	return c.conn.Begin() //nolint:staticcheck
}

func (c *retryingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	qc, ok := c.conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := qc.QueryContext(ctx, query, args)
	if isPoolerArtifact(err) {
		return nil, driver.ErrBadConn
	}
	return rows, err
}

func (c *retryingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	ec, ok := c.conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	res, err := ec.ExecContext(ctx, query, args)
	if isPoolerArtifact(err) {
		return nil, driver.ErrBadConn
	}
	return res, err
}

// Query and Exec (the pre-context driver.Queryer/driver.Execer
// interfaces) are implemented for completeness and the same
// pooler-artifact translation, though modern database/sql always
// prefers the *Context variants above when a driver implements them,
// as lib/pq's connection does.
func (c *retryingConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	q, ok := c.conn.(driver.Queryer) //nolint:staticcheck
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := q.Query(query, args)
	if isPoolerArtifact(err) {
		return nil, driver.ErrBadConn
	}
	return rows, err
}

func (c *retryingConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	e, ok := c.conn.(driver.Execer) //nolint:staticcheck
	if !ok {
		return nil, driver.ErrSkip
	}
	res, err := e.Exec(query, args)
	if isPoolerArtifact(err) {
		return nil, driver.ErrBadConn
	}
	return res, err
}

func (c *retryingConn) Ping(ctx context.Context) error {
	if p, ok := c.conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *retryingConn) ResetSession(ctx context.Context) error {
	if r, ok := c.conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *retryingConn) IsValid() bool {
	if v, ok := c.conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

// CheckNamedValue forwards to lib/pq's own value conversion so
// driver-specific types (e.g. pq.Array, pq.StringArray) keep working
// exactly as they would through the unwrapped "postgres" driver.
func (c *retryingConn) CheckNamedValue(nv *driver.NamedValue) error {
	if chk, ok := c.conn.(driver.NamedValueChecker); ok {
		return chk.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

var (
	_ driver.Conn               = (*retryingConn)(nil)
	_ driver.ConnPrepareContext = (*retryingConn)(nil)
	_ driver.ConnBeginTx        = (*retryingConn)(nil)
	_ driver.QueryerContext     = (*retryingConn)(nil)
	_ driver.ExecerContext      = (*retryingConn)(nil)
	_ driver.Pinger             = (*retryingConn)(nil)
	_ driver.SessionResetter    = (*retryingConn)(nil)
	_ driver.Validator          = (*retryingConn)(nil)
	_ driver.NamedValueChecker  = (*retryingConn)(nil)
)
