package tick

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestCheckForFactionDefeat_ZeroResourcesAndGarrisonMarksDefeated(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionID := seedEncampment(t, db, 52001, "The Ashfall Remnant", 10, 10, "TestRegion", true)

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.checkForFactionDefeat(ctx, tx) })

	var isDefeated bool
	var defeatedAt sql.NullTime
	var epitaph sql.NullString
	if err := db.QueryRow("SELECT is_defeated, defeated_at, epitaph FROM encampments WHERE id = $1", factionID).Scan(&isDefeated, &defeatedAt, &epitaph); err != nil {
		t.Fatalf("reading faction: %v", err)
	}
	if !isDefeated {
		t.Error("expected the faction to be marked defeated")
	}
	if !defeatedAt.Valid {
		t.Error("expected defeated_at to be set")
	}
	if !epitaph.Valid || epitaph.String == "" {
		t.Error("expected a non-empty epitaph to be generated")
	}
}

func TestCheckForFactionDefeat_MerelyPoorButGarrisonedIsSpared(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionID := seedEncampment(t, db, 52002, "The Rustbelt Compact", 11, 11, "TestRegion", true)
	if _, err := db.Exec("UPDATE workshop_inventory SET soldiers = 5 WHERE encampment_id = $1", factionID); err != nil {
		t.Fatalf("seeding garrison: %v", err)
	}

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.checkForFactionDefeat(ctx, tx) })

	var isDefeated bool
	if err := db.QueryRow("SELECT is_defeated FROM encampments WHERE id = $1", factionID).Scan(&isDefeated); err != nil {
		t.Fatalf("reading faction: %v", err)
	}
	if isDefeated {
		t.Error("expected a still-garrisoned faction to NOT be marked defeated, even with zero resources")
	}
}

func TestCheckForFactionDefeat_UngarrisonedButRichIsSpared(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionID := seedEncampment(t, db, 52003, "The Ferrous Cartel", 12, 12, "TestRegion", true)
	if _, err := db.Exec("UPDATE resources SET dollars = 10000 WHERE encampment_id = $1", factionID); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.checkForFactionDefeat(ctx, tx) })

	var isDefeated bool
	if err := db.QueryRow("SELECT is_defeated FROM encampments WHERE id = $1", factionID).Scan(&isDefeated); err != nil {
		t.Fatalf("reading faction: %v", err)
	}
	if isDefeated {
		t.Error("expected a resource-rich but ungarrisoned faction to NOT be marked defeated")
	}
}

func TestCheckForFactionDefeat_IsIdempotent(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionID := seedEncampment(t, db, 52004, "The Voidmark Legion", 13, 13, "TestRegion", true)

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.checkForFactionDefeat(ctx, tx) })

	var firstEpitaph string
	if err := db.QueryRow("SELECT epitaph FROM encampments WHERE id = $1", factionID).Scan(&firstEpitaph); err != nil {
		t.Fatalf("reading first epitaph: %v", err)
	}

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.checkForFactionDefeat(ctx, tx) })

	var secondEpitaph string
	if err := db.QueryRow("SELECT epitaph FROM encampments WHERE id = $1", factionID).Scan(&secondEpitaph); err != nil {
		t.Fatalf("reading second epitaph: %v", err)
	}
	if firstEpitaph != secondEpitaph {
		t.Errorf("expected the epitaph to stay stable across passes, got %q then %q", firstEpitaph, secondEpitaph)
	}
}

func TestCheckForFactionDefeat_CreditsMostRecentAttackerAndPaysConqueror(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionID := seedEncampment(t, db, 52005, "The Hollowpoint Warband", 14, 14, "TestRegion", true)
	attackerID := seedEncampment(t, db, 52006, "Finisher Camp", 15, 15, "TestRegion", false)
	if _, err := db.Exec("UPDATE encampments SET extension_lvl = 40 WHERE id = $1", attackerID); err != nil {
		t.Fatalf("raising attacker storage cap: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time)
		VALUES ($1, $2, 'completed', CURRENT_TIMESTAMP - INTERVAL '1 minute')`, attackerID, factionID); err != nil {
		t.Fatalf("seeding completed raid: %v", err)
	}

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.checkForFactionDefeat(ctx, tx) })

	var epitaph string
	if err := db.QueryRow("SELECT epitaph FROM encampments WHERE id = $1", factionID).Scan(&epitaph); err != nil {
		t.Fatalf("reading epitaph: %v", err)
	}
	if !strings.Contains(epitaph, "Finisher Camp") {
		t.Errorf("expected the epitaph to credit the finishing attacker by name, got %q", epitaph)
	}

	var attackerDollars float64
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", attackerID).Scan(&attackerDollars); err != nil {
		t.Fatalf("reading attacker resources: %v", err)
	}
	wantDollars := conquerorBonusDollars + 2000
	if attackerDollars != wantDollars {
		t.Errorf("expected the attacker to receive %v (conqueror bonus + conqueror achievement), got %v", wantDollars, attackerDollars)
	}
}

func TestCheckForFactionDefeat_NoRaidHistoryFallsBackToFlavor(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionID := seedEncampment(t, db, 52007, "The Emberline Dominion", 16, 16, "TestRegion", true)

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.checkForFactionDefeat(ctx, tx) })

	var epitaph string
	if err := db.QueryRow("SELECT epitaph FROM encampments WHERE id = $1", factionID).Scan(&epitaph); err != nil {
		t.Fatalf("reading epitaph: %v", err)
	}
	if !strings.Contains(epitaph, "the wasteland itself") {
		t.Errorf("expected the epitaph to fall back to \"the wasteland itself\" with no raid history, got %q", epitaph)
	}
}

func TestFactionDefeat_ExcludedFromAIPools(t *testing.T) {
	db := testDB(t)
	factionID := seedEncampment(t, db, 52008, "The Graywater Syndicate", 17, 17, "TestRegion", true)
	if _, err := db.Exec("UPDATE encampments SET is_defeated = TRUE, defeated_at = CURRENT_TIMESTAMP, epitaph = 'test' WHERE id = $1", factionID); err != nil {
		t.Fatalf("marking faction defeated: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM encampments WHERE id = $1 AND is_ai_faction = TRUE AND is_defeated = FALSE", factionID).Scan(&count); err != nil {
		t.Fatalf("querying pool-exclusion condition: %v", err)
	}
	if count != 0 {
		t.Error("expected a defeated faction to be excluded by the is_ai_faction=TRUE AND is_defeated=FALSE filter every AI pool query now uses")
	}

	var stillExists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM encampments WHERE id = $1)", factionID).Scan(&stillExists); err != nil {
		t.Fatalf("checking row existence: %v", err)
	}
	if !stillExists {
		t.Error("expected the defeated faction's row to persist permanently, not be deleted")
	}
}

func TestResolveExplorationDiscovery_DefeatedFactionShowsRuinsAndEpitaph(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	observerID := seedEncampment(t, db, 52101, "Explorer Camp", 20, 20, "TestRegion", false)
	targetID := seedEncampment(t, db, 52102, "The Nightshade Pact", 21, 21, "TestRegion", true)
	const testEpitaph = "Here fell The Nightshade Pact, broken at last by Explorer Camp."
	if _, err := db.Exec("UPDATE encampments SET is_defeated = TRUE, defeated_at = CURRENT_TIMESTAMP, epitaph = $1 WHERE id = $2", testEpitaph, targetID); err != nil {
		t.Fatalf("marking target defeated: %v", err)
	}

	var message string
	for attempt := 0; attempt < 10; attempt++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		msg, discErr := e.resolveExplorationDiscovery(ctx, tx, observerID, "TestRegion")
		if discErr != nil {
			_ = tx.Rollback()
			t.Fatalf("resolveExplorationDiscovery: %v", discErr)
		}
		if commitErr := tx.Commit(); commitErr != nil {
			t.Fatalf("commit: %v", commitErr)
		}
		if msg != "" {
			message = msg
			break
		}
	}

	if message == "" {
		t.Fatal("expected a discovery message within 10 attempts at ~92% base chance")
	}
	if !strings.Contains(message, "RUINS DISCOVERED") {
		t.Errorf("expected the discovery message to use the ruins framing, got %q", message)
	}
	if !strings.Contains(message, testEpitaph) {
		t.Errorf("expected the discovery message to include the generated epitaph, got %q", message)
	}
}
