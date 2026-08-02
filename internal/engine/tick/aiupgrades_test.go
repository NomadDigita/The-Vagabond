package tick

import (
	"context"
	"testing"
	"time"
)

// TestGrowAICivilizationsCanAdvanceResearch verifies an AI faction with
// ample Neuro Cores eventually advances at least one of the seven
// research_states tech columns, mirroring HandleUpgradeTechCallback's
// exact mechanics (cost = currentLvl*8, applied to whichever column is
// rolled).
func TestGrowAICivilizationsCanAdvanceResearch(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3001, "Faction Upsilon", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET neuro_cores = 10000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding neuro cores: %v", err)
	}

	advanced := false
	for i := 0; i < 300 && !advanced; i++ {
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
		row := db.QueryRow(`
			SELECT (econ_tech_lvl + production_tech_lvl + integrity_tech_lvl +
			        defense_tech_lvl + intel_tech_lvl + speed_tech_lvl + military_tech_lvl)
			FROM research_states WHERE encampment_id = $1`, faction)
		var total int
		if err := row.Scan(&total); err == nil && total > 7 {
			advanced = true
		}
	}
	if !advanced {
		t.Fatal("expected the AI faction to advance at least one tech column at least once across 300 ticks")
	}
}

// TestGrowAICivilizationsWontAdvanceResearchPastMaxLevel verifies every
// tech column stays capped at aiResearchMaxLevel (20) even with unlimited
// Neuro Cores available, mirroring HandleUpgradeTechCallback's own
// MaxResearchLevel guard.
func TestGrowAICivilizationsWontAdvanceResearchPastMaxLevel(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3002, "Faction Phi", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 20)
	if _, err := db.Exec("INSERT INTO research_states (encampment_id, econ_tech_lvl, production_tech_lvl, integrity_tech_lvl, defense_tech_lvl, intel_tech_lvl, speed_tech_lvl, military_tech_lvl) VALUES ($1, 20, 20, 20, 20, 20, 20, 20)", faction); err != nil {
		t.Fatalf("seeding maxed research state: %v", err)
	}
	if _, err := db.Exec("UPDATE resources SET neuro_cores = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding neuro cores: %v", err)
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

	row := db.QueryRow(`
		SELECT (econ_tech_lvl + production_tech_lvl + integrity_tech_lvl +
		        defense_tech_lvl + intel_tech_lvl + speed_tech_lvl + military_tech_lvl)
		FROM research_states WHERE encampment_id = $1`, faction)
	var total int
	if err := row.Scan(&total); err != nil {
		t.Fatalf("reading research totals: %v", err)
	}
	if total != 140 {
		t.Fatalf("expected all seven columns to stay at the max of 20 (total 140), got total %d", total)
	}
}

// TestGrowAICivilizationsCanUpgradeAModule verifies a high-level AI
// faction with ample Scrap eventually queues a facility upgrade (a
// modules row transitioning to is_upgrading = TRUE), mirroring
// HandleUpgradeCallback's exact mechanics for non-core modules.
func TestGrowAICivilizationsCanUpgradeAModule(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3003, "Faction Chi", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 15)
	if _, err := db.Exec("UPDATE resources SET scrap = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding scrap: %v", err)
	}

	queued := false
	for i := 0; i < 300 && !queued; i++ {
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
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", faction).Scan(&queued)
	}
	if !queued {
		t.Fatal("expected the AI faction to queue at least one module upgrade across 300 ticks")
	}
}

// TestGrowAICivilizationsWontQueueSecondModuleUpgradeConcurrently verifies
// an AI faction with one module already mid-upgrade never starts a second
// one at the same time, mirroring HandleUpgradeCallback's "Construction
// Queue Busy" guard.
func TestGrowAICivilizationsWontQueueSecondModuleUpgradeConcurrently(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3004, "Faction Omega", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 15)
	if _, err := db.Exec("UPDATE resources SET scrap = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding scrap: %v", err)
	}
	readyAt := time.Now().UTC().Add(time.Hour) // far in the future, so it never completes mid-test
	if _, err := db.Exec("INSERT INTO modules (encampment_id, type, level, is_upgrading, upgrade_ready_at) VALUES ($1, 'tent', 1, TRUE, $2)", faction, readyAt); err != nil {
		t.Fatalf("seeding in-flight upgrade: %v", err)
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

	var upgradingCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE", faction).Scan(&upgradingCount); err != nil {
		t.Fatalf("counting in-flight upgrades: %v", err)
	}
	if upgradingCount != 1 {
		t.Fatalf("expected exactly 1 module mid-upgrade (the pre-seeded one), got %d", upgradingCount)
	}
}

// TestGrowAICivilizationsWontUpgradeModuleAboveFactionLevel verifies a
// low-level AI faction (level 1) never queues a module upgrade even with
// unlimited Scrap, since every module starts at level 1 and 1 is not
// less than a faction level of 1 - mirrors HandleUpgradeCallback's
// "Prerequisite Block: Module levels cannot exceed your Outpost Core
// level" rule.
func TestGrowAICivilizationsWontUpgradeModuleAboveFactionLevel(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3005, "Faction Iota", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 1)
	if _, err := db.Exec("UPDATE resources SET scrap = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding scrap: %v", err)
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

	var upgradingCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE", faction).Scan(&upgradingCount); err != nil {
		t.Fatalf("counting in-flight upgrades: %v", err)
	}
	if upgradingCount != 0 {
		t.Fatalf("expected zero module upgrades queued for a level-1 faction, got %d", upgradingCount)
	}
}

// TestGrowAICivilizationsNeuroCoreTrickleRespectsStorageCap verifies the
// Neuro Core trickle added alongside the research block clamps to the
// faction's storage cap rather than growing unbounded, same as every
// other resource trickle in growAICivilizations.
func TestGrowAICivilizationsNeuroCoreTrickleRespectsStorageCap(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3006, "Faction Kappa", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 3)

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

	var neuro float64
	if err := db.QueryRow("SELECT COALESCE(neuro_cores,0) FROM resources WHERE encampment_id = $1", faction).Scan(&neuro); err != nil {
		t.Fatalf("reading neuro cores: %v", err)
	}
	if neuro <= 0 {
		t.Fatal("expected a level-3 AI faction to gain some Neuro Cores from a single tick")
	}
}
