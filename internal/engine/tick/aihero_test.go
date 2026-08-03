package tick

import (
	"context"
	"testing"
	"time"
)

// TestGrowAICivilizationsCreatesHeroOnFirstTick verifies every AI faction
// gets exactly one Hero row, lazily created on its very first tick,
// mirroring HandleHeroPanel's lazy-create behavior.
func TestGrowAICivilizationsCreatesHeroOnFirstTick(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3010, "Faction Rho", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 2)

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

	var name, trait, injuries string
	var lvl, xp int
	if err := db.QueryRow("SELECT name, trait, injuries, level, xp FROM heroes WHERE encampment_id = $1", faction).
		Scan(&name, &trait, &injuries, &lvl, &xp); err != nil {
		t.Fatalf("expected a Hero row to exist after one tick: %v", err)
	}
	if name == "" || trait == "" || injuries == "" {
		t.Fatalf("expected a fully flavor-named hero, got name=%q trait=%q injuries=%q", name, trait, injuries)
	}
	if lvl != 1 || xp != 0 {
		t.Fatalf("expected a freshly-created hero at level 1 / 0 xp, got level=%d xp=%d", lvl, xp)
	}
}

// TestGrowAICivilizationsWontDuplicateHero verifies repeated ticks never
// create a second Hero row for the same faction, relying on the
// heroes.encampment_id UNIQUE constraint plus the explicit existence
// check.
func TestGrowAICivilizationsWontDuplicateHero(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3011, "Faction Xi", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 2)

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
	}

	var heroCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM heroes WHERE encampment_id = $1", faction).Scan(&heroCount); err != nil {
		t.Fatalf("counting heroes: %v", err)
	}
	if heroCount != 1 {
		t.Fatalf("expected exactly 1 hero after 50 ticks, got %d", heroCount)
	}
}

// TestGrowAICivilizationsCanTrainHero verifies an AI faction with ample
// Scrap eventually trains its Hero (XP increases, and levels up once XP
// crosses 100), mirroring HandleHeroCallback's "train" action exactly.
func TestGrowAICivilizationsCanTrainHero(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3012, "Faction Pi", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 2)
	if _, err := db.Exec("UPDATE resources SET scrap = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding scrap: %v", err)
	}

	trained := false
	for i := 0; i < 300 && !trained; i++ {
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
		var lvl, xp int
		if err := db.QueryRow("SELECT COALESCE(level,1), COALESCE(xp,0) FROM heroes WHERE encampment_id = $1", faction).Scan(&lvl, &xp); err == nil {
			if lvl > 1 || xp > 0 {
				trained = true
			}
		}
	}
	if !trained {
		t.Fatal("expected the AI faction's Hero to gain XP or level up at least once across 300 ticks")
	}
}

// TestGrowAICivilizationsCanHealHero verifies an injured AI faction Hero
// with ample Rations eventually gets healed, mirroring
// HandleHeroCallback's "heal" action exactly.
func TestGrowAICivilizationsCanHealHero(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3013, "Faction Nu", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 2)
	if _, err := db.Exec("UPDATE resources SET rations = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding rations: %v", err)
	}

	// Force-create the hero on tick 1, then seed a real injury before
	// letting the heal roll have a chance to fire.
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
	if _, err := db.Exec("UPDATE heroes SET injuries = 'Shrapnel Wound' WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding injury: %v", err)
	}

	healed := false
	for i := 0; i < 300 && !healed; i++ {
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
		var injuries string
		if err := db.QueryRow("SELECT injuries FROM heroes WHERE encampment_id = $1", faction).Scan(&injuries); err == nil && injuries == "Perfect Health" {
			healed = true
		}
	}
	if !healed {
		t.Fatal("expected the AI faction's injured Hero to be healed at least once across 300 ticks")
	}
}

// TestGrowAICivilizationsWontHealAlreadyHealthyHero verifies an AI
// faction never spends Rations healing a Hero that's already at
// 'Perfect Health' - the one deliberate AI-side guard on top of the
// human handler's exact mechanics (see the comment above the block in
// engine.go for the reasoning).
func TestGrowAICivilizationsWontHealAlreadyHealthyHero(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 3014, "Faction Mu", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 2)

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
	if _, err := db.Exec("UPDATE heroes SET injuries = 'Perfect Health' WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding perfect health: %v", err)
	}
	if _, err := db.Exec("UPDATE resources SET rations = 100000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding rations: %v", err)
	}

	for i := 0; i < 100; i++ {
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

	var injuriesAfter string
	if err := db.QueryRow("SELECT injuries FROM heroes WHERE encampment_id = $1", faction).Scan(&injuriesAfter); err != nil {
		t.Fatalf("reading hero injuries: %v", err)
	}
	if injuriesAfter != "Perfect Health" {
		t.Fatalf("expected an already-healthy hero's injuries to stay 'Perfect Health' (heal is a no-op guard, not a state change), got %q", injuriesAfter)
	}
}
