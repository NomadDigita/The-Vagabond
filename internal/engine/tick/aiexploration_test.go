package tick

import (
	"context"
	"testing"
	"time"
)

// TestGrowAICivilizationsCanDispatchExploration verifies an AI faction
// with ample Rations/Metal eventually dispatches a personal exploration
// expedition, mirroring HandleDispatchExpeditionCallback's exact
// mechanics (site + dispatch rows inserted, resources debited).
func TestGrowAICivilizationsCanDispatchExploration(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3007, "Faction Lambda", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 4)
	if _, err := db.Exec("UPDATE resources SET rations = 100000, metal = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding rations/metal: %v", err)
	}

	dispatched := false
	for i := 0; i < 300 && !dispatched; i++ {
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
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM exploration_dispatches WHERE encampment_id = $1)", faction).Scan(&dispatched)
	}
	if !dispatched {
		t.Fatal("expected the AI faction to dispatch at least one exploration expedition across 300 ticks")
	}

	var rations, metal float64
	if err := db.QueryRow("SELECT rations, metal FROM resources WHERE encampment_id = $1", faction).Scan(&rations, &metal); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if rations >= 100000 || metal >= 100000 {
		t.Fatalf("expected the dispatch to debit Rations/Metal, got rations=%v metal=%v", rations, metal)
	}

	var siteExists bool
	if err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM exploration_dispatches d
			JOIN exploration_sites s ON s.id = d.site_id
			WHERE d.encampment_id = $1
		)`, faction).Scan(&siteExists); err != nil {
		t.Fatalf("checking exploration site: %v", err)
	}
	if !siteExists {
		t.Fatal("expected a real exploration_sites row backing the dispatch")
	}
}

// TestGrowAICivilizationsWontDispatchExplorationConcurrently verifies an
// AI faction with one expedition already en route never starts a second
// one at the same time, mirroring HandleDispatchExpeditionCallback's
// "already have an expedition en route" guard.
func TestGrowAICivilizationsWontDispatchExplorationConcurrently(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3008, "Faction Sigma", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 4)
	if _, err := db.Exec("UPDATE resources SET rations = 100000, metal = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding rations/metal: %v", err)
	}

	var siteID string
	if err := db.QueryRow(`
		INSERT INTO exploration_sites (continent, site_name, site_type, reward_type, reward_amount, expires_at)
		VALUES ('TestRegion', 'Pre-seeded Site', 'Cache', 'metal', 200, $1)
		RETURNING id`, time.Now().UTC().Add(24*time.Hour)).Scan(&siteID); err != nil {
		t.Fatalf("seeding exploration site: %v", err)
	}
	if _, err := db.Exec("INSERT INTO exploration_dispatches (site_id, encampment_id, user_id, resolve_time) VALUES ($1, $2, $3, $4)",
		siteID, faction, 3008, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("seeding in-flight dispatch: %v", err)
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
	}

	var dispatchCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM exploration_dispatches WHERE encampment_id = $1", faction).Scan(&dispatchCount); err != nil {
		t.Fatalf("counting dispatches: %v", err)
	}
	if dispatchCount != 1 {
		t.Fatalf("expected exactly 1 dispatch (the pre-seeded one), got %d", dispatchCount)
	}
}

// TestGrowAICivilizationsWontDispatchExplorationWithoutSupplies verifies
// an AI faction with insufficient Rations/Metal never dispatches an
// exploration expedition, mirroring HandleDispatchExpeditionCallback's
// "Insufficient supplies" guard.
func TestGrowAICivilizationsWontDispatchExplorationWithoutSupplies(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3009, "Faction Tau", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 1)
	if _, err := db.Exec("UPDATE resources SET rations = 0, metal = 0, scrap = 0 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("zeroing resources: %v", err)
	}

	for i := 0; i < 50; i++ {
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
		// Zero out the level-1 Scrap/Rations/Metal trickle after every
		// tick so this faction never accumulates enough to afford the
		// 30 Rations / 15 Metal dispatch cost across all 50 iterations.
		if _, err := db.Exec("UPDATE resources SET rations = 0, metal = 0 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("re-zeroing resources: %v", err)
		}
	}

	var dispatchCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM exploration_dispatches WHERE encampment_id = $1", faction).Scan(&dispatchCount); err != nil {
		t.Fatalf("counting dispatches: %v", err)
	}
	if dispatchCount != 0 {
		t.Fatalf("expected zero exploration dispatches without supplies, got %d", dispatchCount)
	}
}
