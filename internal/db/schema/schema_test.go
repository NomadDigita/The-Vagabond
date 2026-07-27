package schema

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// testDB opens a connection to a real Postgres instance for integration
// testing. Phase 7 milestone 5 explicitly asks for tests covering
// "migration compatibility, critical idempotency paths" - that can only be
// verified against a real database engine, since IF NOT EXISTS / ADD
// COLUMN IF NOT EXISTS semantics and constraint-name collisions are
// Postgres-specific behavior no pure-Go mock can stand in for.
//
// Skips (not fails) when SCHEMA_TEST_DATABASE_URL isn't set, so this
// doesn't break `go test ./...` in environments without a database
// available (e.g. CI without a Postgres service, or a quick local run).
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database schema test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("pinging test database: %v", err)
	}
	return db
}

// TestStatementsNonEmpty is a cheap sanity check that doesn't need a
// database - if this ever returns an empty slice, every other test in this
// package would pass vacuously and hide a real regression.
func TestStatementsNonEmpty(t *testing.T) {
	stmts := Statements()
	if len(stmts) == 0 {
		t.Fatal("Statements() returned no statements")
	}
	for i, s := range stmts {
		if s == "" {
			t.Fatalf("Statements()[%d] is an empty string", i)
		}
	}
}

// TestStatementsRunRepeatedlyCleanly is the actual idempotency test: every
// statement must succeed on a bare database (pass 1, matching a fresh
// deploy), again against a database that already has the full schema
// applied (pass 2, matching every subsequent boot), and a third time for
// good measure - guarding against subtler non-idempotency that only shows
// up once a constraint-guard row or sequence side-effect from a prior pass
// is already present, not just "CREATE TABLE IF NOT EXISTS ran twice".
func TestStatementsRunRepeatedlyCleanly(t *testing.T) {
	db := testDB(t)
	stmts := Statements()

	for pass := 1; pass <= 3; pass++ {
		for i, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				t.Fatalf("pass %d: statement %d failed (not idempotent): %v\n--- statement ---\n%s", pass, i, err, stmt)
			}
		}
	}
}
