package schema

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

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
