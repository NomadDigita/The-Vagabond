package notifications

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/NomadDigita/The-Vagabond/internal/db/schema"
)

// testDB applies the full real schema (not a hand-rolled subset) to a
// throwaway Postgres, so these tests exercise notification_preferences
// exactly as it's actually defined, not a copy that could quietly drift
// from the real migration.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database notification test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range schema.Statements() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("applying schema: %v\n%s", err, stmt)
		}
	}
	return db
}

func seedUser(t *testing.T, db *sql.DB, telegramID int64) {
	t.Helper()
	_, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID)
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
}

// TestMutableCategoriesIsTheOnlyGate is the core safety property Phase 7
// milestone 2 requires: "without suppressing combat, discovery, or
// supply-loss alerts". This doesn't touch a database at all - it checks
// the actual policy object every other test and every call site relies on,
// so a careless future edit that adds "combat" to the map gets caught
// here specifically, not just incidentally by some other test failing.
func TestMutableCategoriesIsTheOnlyGate(t *testing.T) {
	neverMutable := []string{"general", "combat", "discovery", "supply_loss", "raid", ""}
	for _, cat := range neverMutable {
		if MutableCategories[cat] {
			t.Errorf("category %q must never be mutable, but MutableCategories[%q] is true", cat, cat)
		}
	}
	if !MutableCategories["route_status"] {
		t.Error("route_status should be mutable - it's the one category this feature exists for")
	}
}

func TestIsCategoryMutedIgnoresNonMutableCategoriesEvenIfPreferenceRowSaysOtherwise(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const uid int64 = 9001
	seedUser(t, db, uid)

	// Even if some future bug wrote a preference row that a non-mutable
	// category should be muted, IsCategoryMuted must not honor it - the
	// MutableCategories check has to happen before any row is even
	// consulted, per the doc comment on IsCategoryMuted.
	if _, err := db.Exec("INSERT INTO notification_preferences (user_id, mute_route_status) VALUES ($1, TRUE)", uid); err != nil {
		t.Fatalf("seeding preference: %v", err)
	}

	for _, cat := range []string{"general", "combat", "discovery", "supply_loss"} {
		if IsCategoryMuted(ctx, db, uid, cat) {
			t.Errorf("category %q must never report muted, regardless of DB state", cat)
		}
	}
}

func TestIsCategoryMutedDefaultsToUnmutedWithNoPreferenceRow(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const uid int64 = 9002
	seedUser(t, db, uid)

	if IsCategoryMuted(ctx, db, uid, "route_status") {
		t.Error("a user with no notification_preferences row should default to unmuted")
	}
}

func TestIsCategoryMutedRespectsToggle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const uid int64 = 9003
	seedUser(t, db, uid)

	_, err := db.Exec(`
		INSERT INTO notification_preferences (user_id, mute_route_status) VALUES ($1, TRUE)
		ON CONFLICT (user_id) DO UPDATE SET mute_route_status = TRUE`, uid)
	if err != nil {
		t.Fatalf("muting: %v", err)
	}
	if !IsCategoryMuted(ctx, db, uid, "route_status") {
		t.Error("expected route_status to be muted after setting mute_route_status = TRUE")
	}

	_, err = db.Exec("UPDATE notification_preferences SET mute_route_status = FALSE WHERE user_id = $1", uid)
	if err != nil {
		t.Fatalf("unmuting: %v", err)
	}
	if IsCategoryMuted(ctx, db, uid, "route_status") {
		t.Error("expected route_status to be unmuted after setting mute_route_status = FALSE")
	}
}

func TestQueueSkipsInsertWhenCategoryIsMutedForThatUser(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const uid int64 = 9004
	seedUser(t, db, uid)
	if _, err := db.Exec("INSERT INTO notification_preferences (user_id, mute_route_status) VALUES ($1, TRUE)", uid); err != nil {
		t.Fatalf("muting: %v", err)
	}

	if err := Queue(ctx, db, uid, "peaceful pass", "route_status"); err != nil {
		t.Fatalf("Queue returned error: %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", uid).Scan(&count)
	if count != 0 {
		t.Errorf("expected no notification row for a muted category, got %d", count)
	}
}

func TestQueueInsertsWhenCategoryIsNotMuted(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const uid int64 = 9005
	seedUser(t, db, uid)

	if err := Queue(ctx, db, uid, "peaceful pass", "route_status"); err != nil {
		t.Fatalf("Queue returned error: %v", err)
	}

	var count int
	var category string
	err := db.QueryRow("SELECT COUNT(*), MAX(category) FROM notifications WHERE user_id = $1", uid).Scan(&count, &category)
	if err != nil {
		t.Fatalf("querying: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 notification row, got %d", count)
	}
	if category != "route_status" {
		t.Errorf("expected category 'route_status', got %q", category)
	}
}

func TestQueueIgnoresMuteForNonMutableCategory(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const uid int64 = 9006
	seedUser(t, db, uid)
	// mute_route_status is irrelevant here - "general" isn't in
	// MutableCategories at all, so it must always be delivered.
	if _, err := db.Exec("INSERT INTO notification_preferences (user_id, mute_route_status) VALUES ($1, TRUE)", uid); err != nil {
		t.Fatalf("muting: %v", err)
	}

	if err := Queue(ctx, db, uid, "you were raided", "general"); err != nil {
		t.Fatalf("Queue returned error: %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND category = 'general'", uid).Scan(&count)
	if count != 1 {
		t.Errorf("expected a 'general' notification to always be queued regardless of route_status mute, got count %d", count)
	}
}
