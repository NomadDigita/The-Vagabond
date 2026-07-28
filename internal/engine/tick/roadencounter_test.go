package tick

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/NomadDigita/The-Vagabond/internal/db/schema"
)

// testDB applies the full real schema to a throwaway Postgres. Phase 7
// milestone 5 asks for tests covering "all policy boundaries" - the road-
// encounter tick passes are exactly the code that broke twice this session
// from parallel-branch merge collisions, so they're the highest-value
// target for real (not just gofmt/vet) test coverage.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database tick engine test")
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
	// Every test in this file shares one database (spinning up a fresh
	// Postgres per test would be far slower), so each test must start
	// from a clean slate - otherwise a leftover row from an earlier test
	// (or an earlier test's caught bug) can silently satisfy another
	// test's "did an encounter appear" check without that test's own
	// tick call having done anything at all.
	_, err = db.Exec(`TRUNCATE road_encounters, road_base_encounters, raids, encampment_discoveries,
		resources, encampments, coordinates, users RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

// seedEncampment creates a user + encampment + coordinate at (x, y) in
// region and returns the encampment id.
func seedEncampment(t *testing.T, db *sql.DB, telegramID int64, name string, x, y int, region string, isAIFaction bool) string {
	t.Helper()
	_, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID)
	if err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	err = db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $2, 'plains', $3, 'flat')
		ON CONFLICT (x, y) DO UPDATE SET biome = EXCLUDED.biome
		RETURNING id`, x, y, region).Scan(&coordID)
	if err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	err = db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id, is_ai_faction) VALUES ($1, $2, $3, $4)
		RETURNING id`, telegramID, name, coordID, isAIFaction).Scan(&campID)
	if err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	_, _ = db.Exec("INSERT INTO resources (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)
	_, _ = db.Exec("INSERT INTO workshop_inventory (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)
	return campID
}

// seedMarchingRaid creates a raid currently halfway between (ox,oy) and
// (dx,dy) - i.e. it will be found at the midpoint - eligible for both
// evaluateRoadEncounters and evaluateRoadBaseEncounters.
func seedMarchingRaid(t *testing.T, db *sql.DB, attackerID string, ox, oy, dx, dy int, region string) string {
	t.Helper()
	legMinutes := 20.0
	legStarted := time.Now().UTC().Add(-time.Duration(legMinutes/2) * time.Minute) // exactly 50% through
	var raidID string
	err := db.QueryRow(`
		INSERT INTO raids (attacker_id, state, resolve_time, movement_state,
			leg_started_at, leg_total_minutes, origin_x, origin_y, destination_x, destination_y,
			origin_region, destination_region)
		VALUES ($1, 'marching', $2, 'moving', $3, $4, $5, $6, $7, $8, $9, $9)
		RETURNING id`,
		attackerID, time.Now().UTC().Add(time.Duration(legMinutes/2)*time.Minute),
		legStarted, legMinutes, ox, oy, dx, dy, region).Scan(&raidID)
	if err != nil {
		t.Fatalf("seeding marching raid: %v", err)
	}
	return raidID
}

// runUntilCondition repeatedly calls fn (each in its own transaction)
// until done() reports true or the retry budget is exhausted. Necessary
// because the actual encounter roll is probabilistic
// (roadcombat.EncounterRollChance = 0.35, not 1.0) - the production code
// has no injectable RNG, so a bounded retry loop is the honest way to test
// it without either flaking or weakening the real per-tick odds just to
// make a test deterministic. 40 attempts makes the chance of a false
// failure (every attempt missing) about 0.65^40 ≈ 4e-8.
func runUntilCondition(t *testing.T, db *sql.DB, tick func(tx *sql.Tx) error, done func() bool) {
	t.Helper()
	ctx := context.Background()
	for attempt := 0; attempt < 40; attempt++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := tick(tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("tick function returned error: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit tx: %v", err)
		}
		if done() {
			return
		}
	}
	t.Fatalf("condition not met after 40 attempts (EncounterRollChance=0.35, this should be astronomically unlikely to fail legitimately)")
}

func TestEvaluateRoadEncountersFreezesConvergingExpeditions(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)

	campA := seedEncampment(t, db, 1001, "Camp Alpha", 0, 0, "TestRegion", false)
	campB := seedEncampment(t, db, 1002, "Camp Beta", 10, 0, "TestRegion", false)
	raidA := seedMarchingRaid(t, db, campA, 0, 0, 10, 0, "TestRegion") // midpoint (5,0)
	raidB := seedMarchingRaid(t, db, campB, 10, 0, 0, 0, "TestRegion") // midpoint (5,0), converging

	runUntilCondition(t, db,
		func(tx *sql.Tx) error { return e.evaluateRoadEncounters(context.Background(), tx) },
		func() bool {
			var count int
			_ = db.QueryRow("SELECT COUNT(*) FROM road_encounters WHERE raid_a_id IN ($1,$2) AND raid_b_id IN ($1,$2) AND status = 'pending'", raidA, raidB).Scan(&count)
			return count > 0
		})

	// Both raids must be frozen, not just one - this was exactly the kind
	// of asymmetric-freeze bug that a scrambled merge could reintroduce.
	for _, raidID := range []string{raidA, raidB} {
		var movementState string
		var activeEncounterID sql.NullString
		err := db.QueryRow("SELECT movement_state, active_encounter_id FROM raids WHERE id = $1", raidID).Scan(&movementState, &activeEncounterID)
		if err != nil {
			t.Fatalf("querying raid state: %v", err)
		}
		if movementState != "encounter_pending" {
			t.Errorf("raid %s: expected movement_state 'encounter_pending', got %q", raidID, movementState)
		}
		if !activeEncounterID.Valid {
			t.Errorf("raid %s: expected active_encounter_id to be set", raidID)
		}
	}
}

func TestEvaluateRoadEncountersDoesNotPairACommanderWithThemselves(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)

	// Two expeditions from the SAME commander, converging on each other -
	// must never generate a road_encounters row against themselves.
	camp := seedEncampment(t, db, 1003, "Camp Solo", 0, 0, "TestRegion", false)
	seedMarchingRaid(t, db, camp, 0, 0, 10, 0, "TestRegion")
	seedMarchingRaid(t, db, camp, 10, 0, 0, 0, "TestRegion")

	ctx := context.Background()
	for i := 0; i < 40; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.evaluateRoadEncounters(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("evaluateRoadEncounters: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM road_encounters").Scan(&count)
	if count != 0 {
		t.Errorf("expected zero road_encounters for a commander's two columns meeting each other, got %d", count)
	}
}

func TestEvaluateRoadBaseEncountersExcludesAIFactionBases(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)

	attacker := seedEncampment(t, db, 1004, "Camp Raider", 0, 0, "TestRegion", false)
	// An AI-owned base sitting exactly at the raid's midpoint.
	seedEncampment(t, db, 1005, "Rogue Outpost", 5, 0, "TestRegion", true)
	seedMarchingRaid(t, db, attacker, 0, 0, 10, 0, "TestRegion")

	ctx := context.Background()
	for i := 0; i < 40; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.evaluateRoadBaseEncounters(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("evaluateRoadBaseEncounters: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM road_base_encounters").Scan(&count)
	if count != 0 {
		t.Errorf("expected zero road_base_encounters against an AI-faction base, got %d (AI/seeded settlements have their own recon/skirmish flow)", count)
	}
}

func TestEvaluateRoadBaseEncountersFreezesTheColumnAndRecordsMutualDiscovery(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)

	attacker := seedEncampment(t, db, 1006, "Camp Raider Two", 0, 0, "TestRegion", false)
	base := seedEncampment(t, db, 1007, "Camp Passive", 5, 0, "TestRegion", false)
	raidID := seedMarchingRaid(t, db, attacker, 0, 0, 10, 0, "TestRegion")

	runUntilCondition(t, db,
		func(tx *sql.Tx) error { return e.evaluateRoadBaseEncounters(context.Background(), tx) },
		func() bool {
			var count int
			_ = db.QueryRow("SELECT COUNT(*) FROM road_base_encounters WHERE raid_id = $1 AND status = 'pending'", raidID).Scan(&count)
			return count > 0
		})

	var movementState string
	var activeBaseEncounterID sql.NullString
	err := db.QueryRow("SELECT movement_state, active_base_encounter_id FROM raids WHERE id = $1", raidID).Scan(&movementState, &activeBaseEncounterID)
	if err != nil {
		t.Fatalf("querying raid state: %v", err)
	}
	if movementState != "encounter_pending" {
		t.Errorf("expected movement_state 'encounter_pending', got %q", movementState)
	}
	if !activeBaseEncounterID.Valid {
		t.Error("expected active_base_encounter_id to be set")
	}

	// Discovery must be reciprocal even when nobody has decided to fight
	// yet - matches discoverRouteContacts' existing behavior.
	var attackerDiscoveredBase, baseDiscoveredAttacker int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampment_discoveries WHERE observer_encampment_id = $1 AND target_encampment_id = $2", attacker, base).Scan(&attackerDiscoveredBase)
	_ = db.QueryRow("SELECT COUNT(*) FROM encampment_discoveries WHERE observer_encampment_id = $1 AND target_encampment_id = $2", base, attacker).Scan(&baseDiscoveredAttacker)
	if attackerDiscoveredBase == 0 {
		t.Error("expected the attacking expedition to have discovered the base")
	}
	if baseDiscoveredAttacker == 0 {
		t.Error("expected the base to have reciprocally discovered the attacking expedition")
	}
}

func TestExpireRoadBaseEncountersResumesTheColumnFromWhereItPaused(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	attacker := seedEncampment(t, db, 1008, "Camp Raider Three", 0, 0, "TestRegion", false)
	seedEncampment(t, db, 1009, "Camp Passive Two", 5, 0, "TestRegion", false)
	raidID := seedMarchingRaid(t, db, attacker, 0, 0, 10, 0, "TestRegion")

	runUntilCondition(t, db,
		func(tx *sql.Tx) error { return e.evaluateRoadBaseEncounters(ctx, tx) },
		func() bool {
			var count int
			_ = db.QueryRow("SELECT COUNT(*) FROM road_base_encounters WHERE raid_id = $1 AND status = 'pending'", raidID).Scan(&count)
			return count > 0
		})

	// Force the response window to have already lapsed, then run the
	// expiry pass directly - this is the "nobody tapped a button in time"
	// path, distinct from the explicit-Continue path in the bot handler.
	_, err := db.Exec("UPDATE road_base_encounters SET response_deadline = $1 WHERE raid_id = $2 AND status = 'pending'",
		time.Now().UTC().Add(-time.Minute), raidID)
	if err != nil {
		t.Fatalf("forcing deadline: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.expireRoadBaseEncounters(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("expireRoadBaseEncounters: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var movementState string
	var activeBaseEncounterID sql.NullString
	var pausedAt sql.NullTime
	err = db.QueryRow("SELECT movement_state, active_base_encounter_id, paused_at FROM raids WHERE id = $1", raidID).Scan(&movementState, &activeBaseEncounterID, &pausedAt)
	if err != nil {
		t.Fatalf("querying raid state: %v", err)
	}
	if movementState != "moving" {
		t.Errorf("expected the column to resume moving after an unanswered encounter expired, got movement_state %q", movementState)
	}
	if activeBaseEncounterID.Valid {
		t.Error("expected active_base_encounter_id to be cleared after expiry")
	}
	if pausedAt.Valid {
		t.Error("expected paused_at to be cleared after expiry")
	}

	var status, outcome string
	err = db.QueryRow("SELECT status, outcome FROM road_base_encounters WHERE raid_id = $1", raidID).Scan(&status, &outcome)
	if err != nil {
		t.Fatalf("querying encounter status: %v", err)
	}
	if status != "resolved" || outcome != "pass" {
		t.Errorf("expected status='resolved', outcome='pass', got status=%q outcome=%q", status, outcome)
	}
}
