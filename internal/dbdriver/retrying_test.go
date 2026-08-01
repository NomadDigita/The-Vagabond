package dbdriver

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/lib/pq"
)

// fakeConn is a minimal driver.Conn + QueryerContext/ExecerContext
// stub standing in for lib/pq's real *pq.conn, so these tests don't
// need a live Postgres/PgBouncer to prove the translation logic.
type fakeConn struct {
	queryErr error
	execErr  error
}

func (f *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeConn) Close() error              { return nil }
func (f *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") } //nolint:staticcheck

func (f *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return &fakeRows{}, nil
}

func (f *fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if f.execErr != nil {
		return nil, f.execErr
	}
	return driver.RowsAffected(1), nil
}

type fakeRows struct{}

func (r *fakeRows) Columns() []string              { return nil }
func (r *fakeRows) Close() error                   { return nil }
func (r *fakeRows) Next(dest []driver.Value) error { return nil }

func TestQueryContext_TranslatesPoolerArtifactToErrBadConn(t *testing.T) {
	underlying := &fakeConn{queryErr: &pq.Error{Code: pgInvalidSQLStatementName, Message: "unnamed prepared statement does not exist"}}
	c := &retryingConn{conn: underlying}

	_, err := c.QueryContext(context.Background(), "SELECT 1", nil)
	if !errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("expected driver.ErrBadConn so database/sql retries on a fresh connection, got: %v", err)
	}
}

func TestExecContext_TranslatesPoolerArtifactToErrBadConn(t *testing.T) {
	underlying := &fakeConn{execErr: &pq.Error{Code: pgInvalidSQLStatementName, Message: "unnamed prepared statement does not exist"}}
	c := &retryingConn{conn: underlying}

	_, err := c.ExecContext(context.Background(), "UPDATE users SET x = 1", nil)
	if !errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("expected driver.ErrBadConn so database/sql retries on a fresh connection, got: %v", err)
	}
}

// A genuinely different Postgres error (e.g. a unique constraint
// violation) must reach the caller unchanged — retrying it would just
// reproduce the same failure and, worse, would mask a real bug behind
// a silent retry.
func TestQueryContext_PassesThroughUnrelatedPqErrors(t *testing.T) {
	original := &pq.Error{Code: "23505", Message: "duplicate key value violates unique constraint"}
	underlying := &fakeConn{queryErr: original}
	c := &retryingConn{conn: underlying}

	_, err := c.QueryContext(context.Background(), "INSERT INTO users ...", nil)
	if errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("did not expect a real query error to be turned into a retry signal")
	}
	if err != original {
		t.Fatalf("expected the original error to pass through unchanged, got: %v", err)
	}
}

// A non-pq error (e.g. a context deadline) is not a pooler artifact
// either and must also pass through untouched.
func TestQueryContext_PassesThroughNonPqErrors(t *testing.T) {
	original := context.DeadlineExceeded
	underlying := &fakeConn{queryErr: original}
	c := &retryingConn{conn: underlying}

	_, err := c.QueryContext(context.Background(), "SELECT 1", nil)
	if errors.Is(err, driver.ErrBadConn) {
		t.Fatalf("did not expect a non-pq error to be turned into a retry signal")
	}
	if err != original {
		t.Fatalf("expected the original error to pass through unchanged, got: %v", err)
	}
}

func TestQueryContext_SuccessPassesThroughUnchanged(t *testing.T) {
	underlying := &fakeConn{}
	c := &retryingConn{conn: underlying}

	rows, err := c.QueryContext(context.Background(), "SELECT 1", nil)
	if err != nil {
		t.Fatalf("unexpected error on success path: %v", err)
	}
	if rows == nil {
		t.Fatalf("expected rows to be forwarded from the underlying connection")
	}
}

func TestIsPoolerArtifact(t *testing.T) {
	if !isPoolerArtifact(&pq.Error{Code: "26000"}) {
		t.Errorf("expected SQLSTATE 26000 to be recognized as a pooler artifact")
	}
	if isPoolerArtifact(&pq.Error{Code: "23505"}) {
		t.Errorf("expected an unrelated SQLSTATE to not be recognized as a pooler artifact")
	}
	if isPoolerArtifact(errors.New("some other error")) {
		t.Errorf("expected a non-pq error to not be recognized as a pooler artifact")
	}
	if isPoolerArtifact(nil) {
		t.Errorf("expected a nil error to not be recognized as a pooler artifact")
	}
}
