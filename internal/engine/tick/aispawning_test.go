package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestSpawnNewAIFactionsCreatesAFullyFunctionalEncampment verifies a
// spawned faction gets exactly the same shape as a boot-seeded one
// (seedAICivilizations in cmd/bot/main.go): a users row, an encampments
// row with is_ai_faction = TRUE at the deliberately-weak starting level,
// a resources row, and a workshop_inventory row - so growAICivilizations
// and decideAIFactionActions need zero special-casing to pick it up.
func TestSpawnNewAIFactionsCreatesAFullyFunctionalEncampment(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	var beforeCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&beforeCount)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx); err != nil {
		t.Fatalf("spawnNewAIFactions: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var afterCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&afterCount)
	if afterCount != beforeCount+1 {
		t.Fatalf("expected exactly one new AI faction, went from %d to %d", beforeCount, afterCount)
	}

	var campID, aiFactionKey string
	var level int
	var isAI bool
	if err := db.QueryRow(`
		SELECT id, ai_faction_key, level, is_ai_faction FROM encampments
		WHERE is_ai_faction = TRUE ORDER BY established_at DESC LIMIT 1`).
		Scan(&campID, &aiFactionKey, &level, &isAI); err != nil {
		t.Fatalf("querying spawned faction: %v", err)
	}
	if !isAI {
		t.Error("expected the spawned encampment to be flagged is_ai_faction")
	}
	if level != aiFactionSpawnStartingLevel {
		t.Errorf("expected the spawned faction at the deliberately-weak starting level %d, got %d", aiFactionSpawnStartingLevel, level)
	}
	if aiFactionKey == "" {
		t.Error("expected a non-empty ai_faction_key")
	}

	var resourceCount, garrisonCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM resources WHERE encampment_id = $1", campID).Scan(&resourceCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&garrisonCount)
	if resourceCount != 1 {
		t.Errorf("expected exactly one resources row for the spawned faction, got %d", resourceCount)
	}
	if garrisonCount != 1 {
		t.Errorf("expected exactly one workshop_inventory row for the spawned faction, got %d", garrisonCount)
	}

	var lastSpawnAt sql.NullTime
	var spawnedCount int
	_ = db.QueryRow("SELECT last_ai_spawn_at, ai_factions_spawned_count FROM world_state WHERE id = 1").Scan(&lastSpawnAt, &spawnedCount)
	if !lastSpawnAt.Valid {
		t.Error("expected world_state.last_ai_spawn_at to be set after a spawn")
	}
	if spawnedCount != 1 {
		t.Errorf("expected world_state.ai_factions_spawned_count = 1, got %d", spawnedCount)
	}

	var newsCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM world_news WHERE headline LIKE '%NEW SETTLEMENT DETECTED%'").Scan(&newsCount)
	if newsCount != 1 {
		t.Errorf("expected exactly one 'NEW SETTLEMENT DETECTED' world_news headline, got %d", newsCount)
	}
}

// TestSpawnNewAIFactionsRespectsCadence verifies the world-scoped cadence
// gate: calling spawnNewAIFactions again immediately after a spawn does
// nothing until aiFactionSpawnInterval has passed.
func TestSpawnNewAIFactionsRespectsCadence(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx); err != nil {
		t.Fatalf("first spawnNewAIFactions: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var countAfterFirst int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&countAfterFirst)

	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx2); err != nil {
		t.Fatalf("second (should be gated) spawnNewAIFactions: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var countAfterSecond int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&countAfterSecond)
	if countAfterSecond != countAfterFirst {
		t.Errorf("expected the cadence gate to block a second spawn immediately after the first, went from %d to %d", countAfterFirst, countAfterSecond)
	}

	// Force the cadence to have elapsed and confirm a third spawn succeeds.
	if _, err := db.Exec("UPDATE world_state SET last_ai_spawn_at = CURRENT_TIMESTAMP - INTERVAL '1 day' WHERE id = 1"); err != nil {
		t.Fatalf("forcing cadence elapsed: %v", err)
	}
	tx3, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx3); err != nil {
		t.Fatalf("third spawnNewAIFactions: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var countAfterThird int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&countAfterThird)
	if countAfterThird != countAfterFirst+1 {
		t.Errorf("expected a spawn once the cadence had elapsed, went from %d to %d", countAfterFirst, countAfterThird)
	}
}

// TestGrowAICivilizationsCanListSurplusOnExchange verifies an AI faction
// with a healthy crystal surplus eventually lists some of it on
// market_exchange, using the exact same lot size/price a human posting
// via /exchange would - project owner direction 2026-08-01 ("they can
// literally do every single thing in the game," market listings given as
// the explicit example). Runs many ticks since the listing chance is
// probabilistic, mirroring TestAIFactionCanRaidAnotherAIFactionRarely's
// pattern for a low-probability-per-call behavior.
func TestGrowAICivilizationsCanListSurplusOnExchange(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2023, "Faction Mu", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET crystal = 100 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding crystal surplus: %v", err)
	}

	for i := 0; i < 200; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.growAICivilizations(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("growAICivilizations: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		// Replenish crystal so later iterations still have a surplus to
		// list even after an earlier iteration already listed some.
		if _, err := db.Exec("UPDATE resources SET crystal = 100 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("replenishing crystal: %v", err)
		}

		var listingCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM market_exchange WHERE seller_id = $1 AND item_type = 'crystal'", faction).Scan(&listingCount)
		if listingCount > 0 {
			return // success - no need to keep looping
		}
	}
	t.Error("expected the AI faction to list crystal on the exchange at least once across 200 ticks at a 3% chance per tick")
}
