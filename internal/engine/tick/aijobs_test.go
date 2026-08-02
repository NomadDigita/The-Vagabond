package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestRunAIJobsCanGatherSunlight verifies an AI faction eventually gets
// a free Electricity burst, gated by the same 30-minute cooldown a
// human's own repeated taps would hit.
func TestRunAIJobsCanGatherSunlight(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2037, "Faction Omega", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)

	var before float64
	_ = db.QueryRow("SELECT electricity FROM resources WHERE encampment_id = $1", faction).Scan(&before)

	gained := false
	for i := 0; i < 300 && !gained; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.runAIJobs(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("runAIJobs: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		var after float64
		_ = db.QueryRow("SELECT electricity FROM resources WHERE encampment_id = $1", faction).Scan(&after)
		gained = after > before
	}
	if !gained {
		t.Fatal("expected the AI faction to gather sunlight at least once across 300 ticks")
	}

	// Immediately after gaining, the 30-minute cooldown should block a
	// second gain even across many more ticks.
	var afterFirst float64
	_ = db.QueryRow("SELECT electricity FROM resources WHERE encampment_id = $1", faction).Scan(&afterFirst)
	for i := 0; i < 100; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.runAIJobs(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("runAIJobs: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	var afterCooldownWindow float64
	_ = db.QueryRow("SELECT electricity FROM resources WHERE encampment_id = $1", faction).Scan(&afterCooldownWindow)
	if afterCooldownWindow != afterFirst {
		t.Errorf("expected the 30-minute cooldown to block a second gain, but electricity changed from %.2f to %.2f", afterFirst, afterCooldownWindow)
	}
}

// TestRunAIJobsCanRepairUnits verifies an AI faction with enough Scrap
// eventually spends 200 for +5 Soldiers.
func TestRunAIJobsCanRepairUnits(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2038, "Faction Alpha-2", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET scrap = 5000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding scrap: %v", err)
	}
	setGarrison(t, db, faction, 0, 0)

	repaired := false
	for i := 0; i < 300 && !repaired; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.runAIJobs(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("runAIJobs: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if _, err := db.Exec("UPDATE resources SET scrap = 5000 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("replenishing scrap: %v", err)
		}
		var soldiers int
		_ = db.QueryRow("SELECT soldiers FROM workshop_inventory WHERE encampment_id = $1", faction).Scan(&soldiers)
		repaired = soldiers >= 5
	}
	if !repaired {
		t.Fatal("expected the AI faction to repair at least 5 soldiers across 300 ticks")
	}
}

// TestRunAIJobsCanRushBuildingUpgrade verifies an AI faction with a
// module mid-upgrade eventually spends Scrap to halve the remaining time.
func TestRunAIJobsCanRushBuildingUpgrade(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2039, "Faction Beta-2", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET scrap = 5000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding scrap: %v", err)
	}
	originalReady := time.Now().UTC().Add(2 * time.Hour)
	if _, err := db.Exec("INSERT INTO modules (encampment_id, type, level, is_upgrading, upgrade_ready_at) VALUES ($1, 'workshop', 1, TRUE, $2)", faction, originalReady); err != nil {
		t.Fatalf("seeding an in-progress upgrade: %v", err)
	}

	rushed := false
	for i := 0; i < 300 && !rushed; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.runAIJobs(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("runAIJobs: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if _, err := db.Exec("UPDATE resources SET scrap = 5000 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("replenishing scrap: %v", err)
		}
		var readyAt time.Time
		_ = db.QueryRow("SELECT upgrade_ready_at FROM modules WHERE encampment_id = $1", faction).Scan(&readyAt)
		rushed = readyAt.Before(originalReady)
	}
	if !rushed {
		t.Fatal("expected the AI faction to rush the module upgrade at least once across 300 ticks")
	}
}

// TestRunAIJobsCanUseHyperSpeed verifies an AI faction with an active
// outbound raid eventually spends Electricity to halve its remaining
// resolve time.
func TestRunAIJobsCanUseHyperSpeed(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2040, "Faction Gamma-2", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET electricity = 5000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding electricity: %v", err)
	}
	target := seedEncampment(t, db, 2041, "Target Player-2", 1, 0, "TestRegion", false)
	originalResolve := time.Now().UTC().Add(3 * time.Hour)
	if _, err := db.Exec("INSERT INTO raids (attacker_id, defender_id, state, resolve_time) VALUES ($1, $2, 'marching', $3)", faction, target, originalResolve); err != nil {
		t.Fatalf("seeding an active raid: %v", err)
	}

	accelerated := false
	for i := 0; i < 300 && !accelerated; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.runAIJobs(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("runAIJobs: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if _, err := db.Exec("UPDATE resources SET electricity = 5000 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("replenishing electricity: %v", err)
		}
		var resolveTime time.Time
		_ = db.QueryRow("SELECT resolve_time FROM raids WHERE attacker_id = $1", faction).Scan(&resolveTime)
		accelerated = resolveTime.Before(originalResolve)
	}
	if !accelerated {
		t.Fatal("expected the AI faction to use HyperSpeed at least once across 300 ticks")
	}
}

// TestRunAIJobsCanUseOrbitalManeuverAndExtendPlanet verifies the two
// lower-probability actions still fire given enough ticks.
func TestRunAIJobsCanUseOrbitalManeuverAndExtendPlanet(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2042, "Faction Delta-2", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET electricity = 5000, metal = 50000, crystal = 50000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding resources: %v", err)
	}

	buffed, extended := false, false
	for i := 0; i < 1500 && !(buffed && extended); i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.runAIJobs(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("runAIJobs: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if _, err := db.Exec("UPDATE resources SET electricity = 5000, metal = 50000, crystal = 50000 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("replenishing resources: %v", err)
		}
		var buffUntil sql.NullTime
		var extensionLvl int
		_ = db.QueryRow("SELECT orbital_buff_until, COALESCE(extension_lvl,0) FROM encampments WHERE id = $1", faction).Scan(&buffUntil, &extensionLvl)
		buffed = buffUntil.Valid
		extended = extensionLvl > 0
	}
	if !buffed {
		t.Error("expected the AI faction to use Orbital Maneuver at least once across 1500 ticks")
	}
	if !extended {
		t.Error("expected the AI faction to Extend Planet at least once across 1500 ticks")
	}
}
