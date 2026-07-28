package world

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/NomadDigita/The-Vagabond/internal/db/schema"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database weather test")
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
	if _, err := db.Exec(`TRUNCATE world_events, world_news, notifications, encampments, coordinates, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

func seedWeatherPlayer(t *testing.T, db *sql.DB, telegramID int64, x, y int, region string) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $2, 'plains', $3, 'flat')
		ON CONFLICT (x, y) DO UPDATE SET biome = EXCLUDED.biome
		RETURNING id`, x, y, region).Scan(&coordID)
	if err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO encampments (user_id, name, coordinate_id, is_ai_faction) VALUES ($1, $2, $3, FALSE)`,
		telegramID, "WeatherTestCamp", coordID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
}

// TestRunWeatherPass_NewEventNotifiesRegionPlayersDirectly proves a new
// world event does more than write a passive world_news row - it also
// pushes a direct notification to every real player in the affected
// continent, per AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.3.
func TestRunWeatherPass_NewEventNotifiesRegionPlayersDirectly(t *testing.T) {
	db := testDB(t)
	w := NewWeatherEngine(db)
	ctx := context.Background()

	seedWeatherPlayer(t, db, 9001, 500, 500, "Africa")
	seedWeatherPlayer(t, db, 9002, -500, 500, "Europe") // different continent, must not be notified for an Africa event

	// Deterministically force a new event to roll on the very first pass:
	// eventRollChance is 10%, so run the pass in a retry loop rather than
	// depending on a lucky single roll (same probabilistic-but-bounded
	// pattern the tick-engine tests already use elsewhere).
	found := false
	for attempt := 0; attempt < 200 && !found; attempt++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := w.RunWeatherPass(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("RunWeatherPass: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM world_events WHERE continent = 'Africa'").Scan(&count)
		if count > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a world event to have rolled for Africa within 200 attempts at 10% chance")
	}

	var africaNotified int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", int64(9001)).Scan(&africaNotified)
	if africaNotified == 0 {
		t.Error("expected the Africa player to receive a direct notification for the new world event")
	}

	// The Europe player must never receive a notification mentioning
	// Africa - cross-region isolation is already unit-tested directly in
	// QueueToRegion's own tests, so this just confirms weather.go passes
	// the right continent through, not a stale/wrong one.
	var wrongRegionCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message LIKE '%Africa%'", int64(9002)).Scan(&wrongRegionCount)
	if wrongRegionCount != 0 {
		t.Error("expected the Europe player to never receive an Africa-region notification")
	}
}

// TestRunWeatherPass_ClearedEventNotifiesRegionPlayersDirectly covers the
// other half of section 5.3: the "conditions have cleared" headline
// should also reach affected players directly, not just world_news.
func TestRunWeatherPass_ClearedEventNotifiesRegionPlayersDirectly(t *testing.T) {
	db := testDB(t)
	w := NewWeatherEngine(db)
	ctx := context.Background()

	seedWeatherPlayer(t, db, 9101, 500, 500, "Africa")

	if _, err := db.Exec(`
		INSERT INTO world_events (event_type, continent, expires_at)
		VALUES ('solar_flare', 'Africa', $1)`, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("seeding expired world event: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := w.RunWeatherPass(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("RunWeatherPass: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message LIKE '%cleared%'", int64(9101)).Scan(&count)
	if count == 0 {
		t.Error("expected the Africa player to be notified directly that conditions cleared")
	}
}
