package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestRelicConvoySpawn_OnlyOneActiveAtATime is the direct test for
// RARE_WORLD_FEATURES_PLAN.md Phase 1.2's "at most one live at a time,
// server-wide" rule - relicConvoySpawn must never create a second
// convoy while one is already active, even at 100% roll odds.
func TestRelicConvoySpawn_OnlyOneActiveAtATime(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	seedEncampment(t, db, 51001, "Anchor Camp", 0, 0, "TestRegion", false)

	oldChance := relicConvoySpawnChance
	relicConvoySpawnChance = 1.0
	t.Cleanup(func() { relicConvoySpawnChance = oldChance })

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.relicConvoySpawn(ctx, tx) })

	var countAfterFirst int
	if err := db.QueryRow("SELECT COUNT(*) FROM relic_convoys WHERE claimed_by IS NULL AND expires_at > CURRENT_TIMESTAMP").Scan(&countAfterFirst); err != nil {
		t.Fatalf("counting relic convoys: %v", err)
	}
	if countAfterFirst != 1 {
		t.Fatalf("expected exactly 1 active relic convoy after the first spawn phase, got %d", countAfterFirst)
	}

	// Run it again at the same 100% roll odds - the "already active"
	// guard must prevent a second spawn.
	runTickPhase(t, db, func(tx *sql.Tx) error { return e.relicConvoySpawn(ctx, tx) })

	var countAfterSecond int
	if err := db.QueryRow("SELECT COUNT(*) FROM relic_convoys").Scan(&countAfterSecond); err != nil {
		t.Fatalf("counting relic convoys: %v", err)
	}
	if countAfterSecond != 1 {
		t.Errorf("expected the second spawn phase to be a no-op (still 1 total convoy), got %d", countAfterSecond)
	}
}

// TestRelicConvoySpawn_NeverRollsWhenChanceIsZero confirms the roll
// actually gates spawning - at 0% chance, no convoy should ever appear
// even with no active convoy blocking it.
func TestRelicConvoySpawn_NeverRollsWhenChanceIsZero(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	seedEncampment(t, db, 51002, "Anchor Camp 2", 0, 0, "TestRegion", false)

	oldChance := relicConvoySpawnChance
	relicConvoySpawnChance = 0.0
	t.Cleanup(func() { relicConvoySpawnChance = oldChance })

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.relicConvoySpawn(ctx, tx) })

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM relic_convoys").Scan(&count); err != nil {
		t.Fatalf("counting relic convoys: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no relic convoy to spawn at 0%% chance, got %d", count)
	}
}

// TestRelicConvoyExpire_ClearsOnlyUnclaimedExpiredConvoys covers Phase
// 1.4: an unclaimed convoy past its expires_at is deleted, but a
// claimed convoy (kept permanently for the Hall of Relics) and a
// still-live unclaimed convoy are both left untouched.
func TestRelicConvoyExpire_ClearsOnlyUnclaimedExpiredConvoys(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	winnerID := seedEncampment(t, db, 51003, "Winner Camp", 0, 0, "TestRegion", false)

	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES (5, 5, 'plains', 'TestRegion', 'flat')
		RETURNING id`).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}

	// Expired + unclaimed -> should be deleted.
	var expiredID string
	if err := db.QueryRow(`
		INSERT INTO relic_convoys (relic_name, coordinate_id, reward_dollars, expires_at)
		VALUES ('Expired Relic', $1, 1000, CURRENT_TIMESTAMP - INTERVAL '1 minute') RETURNING id`, coordID).Scan(&expiredID); err != nil {
		t.Fatalf("seeding expired convoy: %v", err)
	}
	// Expired + claimed -> must survive (permanent Hall of Relics history).
	var claimedID string
	if err := db.QueryRow(`
		INSERT INTO relic_convoys (relic_name, coordinate_id, reward_dollars, expires_at, claimed_by, claimed_at)
		VALUES ('Claimed Relic', $1, 2000, CURRENT_TIMESTAMP - INTERVAL '1 minute', $2, CURRENT_TIMESTAMP) RETURNING id`, coordID, winnerID).Scan(&claimedID); err != nil {
		t.Fatalf("seeding claimed convoy: %v", err)
	}
	// Still live -> must survive.
	var liveID string
	if err := db.QueryRow(`
		INSERT INTO relic_convoys (relic_name, coordinate_id, reward_dollars, expires_at)
		VALUES ('Live Relic', $1, 3000, CURRENT_TIMESTAMP + INTERVAL '1 hour') RETURNING id`, coordID).Scan(&liveID); err != nil {
		t.Fatalf("seeding live convoy: %v", err)
	}

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.relicConvoyExpire(ctx, tx) })

	var stillExists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM relic_convoys WHERE id = $1)", expiredID).Scan(&stillExists)
	if stillExists {
		t.Error("expected the expired, unclaimed convoy to be deleted")
	}
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM relic_convoys WHERE id = $1)", claimedID).Scan(&stillExists)
	if !stillExists {
		t.Error("expected the claimed convoy to survive expiry (permanent history)")
	}
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM relic_convoys WHERE id = $1)", liveID).Scan(&stillExists)
	if !stillExists {
		t.Error("expected the still-live convoy to survive")
	}
}

// runTickPhase runs a single tick-phase function in its own committed
// transaction - the non-probabilistic counterpart to
// roadencounter_test.go's runUntilCondition, for phases this file's
// tests drive deterministically (via the var-not-const roll-chance
// override above) rather than by retrying a real random roll.
func runTickPhase(t *testing.T, db *sql.DB, phase func(tx *sql.Tx) error) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := phase(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("tick phase returned error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
}
