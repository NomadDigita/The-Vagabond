package dbdriver

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/lib/pq"
)

const pgInvalidSQLStatementName = "26000"

func Register(name string) {
	sql.Register(name, retryingDriver{})
}

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

type retryingConn struct {
	conn driver.Conn
}

func (c *retryingConn) Prepare(query string) (driver.Stmt, error) {
	return c.conn.Prepare(query)
}

func (c *retryingConn) Close() error {
	return c.conn.Close()
}

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
