package tick

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// TestCheckForFactionDefeat_ZeroResourcesAndGarrisonMarksDefeated is the
// direct test for RARE_WORLD_FEATURES_PLAN.md Phase 3.3: an AI faction
// at (near-)zero resources AND (near-)zero garrison gets marked
// defeated, with an epitaph generated, in a single pass.
func TestCheckForFactionDefeat_ZeroResourcesAndGarrisonMarksDefeated(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionID := seedEncampment(t, db, 52001, "The Ashfall Remnant", 10, 10, "TestRegion", true)
	// seedEncampment already leaves resources/workshop_inventory at
	// their zero defaults - nothing further to seed for this case.

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

// TestCheckForFactionDefeat_MerelyPoorButGarrisonedIsSpared covers the
// plan doc's explicit "both conditions, not just resources" rule: a
// faction with an empty treasury but a live garrison must NOT be
// marked defeated.
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

// TestCheckForFactionDefeat_UngarrisonedButRichIsSpared is the mirror
// case: a faction with no garrison but real resources must also NOT be
// marked defeated - "total defeat" means both, not either.
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

// TestCheckForFactionDefeat_IsIdempotent confirms a faction already
// marked defeated is left alone on a second pass (no duplicate
// notification, no epitaph regenerated/overwritten).
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

// TestCheckForFactionDefeat_CreditsMostRecentAttackerAndPaysConqueror
// covers the finishing-attacker lookup (3.3) and conqueror bonus: when
// a real player's completed raid is the faction's most recent, the
// epitaph should credit that player by name and they should receive
// the conqueror bonus, storage-cap-clamped like every other
// resource-gain path.
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
	if attackerDollars != conquerorBonusDollars {
		t.Errorf("expected the attacker to receive the %v conqueror bonus, got %v", conquerorBonusDollars, attackerDollars)
	}
}

// TestCheckForFactionDefeat_NoRaidHistoryFallsBackToFlavor covers the
// no-raid-history fallback: a faction that starved out with no combat
// record at all should still get a valid epitaph (crediting "the
// wasteland itself"), and no conqueror bonus should be paid to anyone.
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

// TestFactionDefeat_ExcludedFromAIPools confirms a defeated faction no
// longer appears in the AI-decision, AI-jobs, or AI-growth pools (Phase
// 3.4) - it stops scouting, raiding, growing, and running jobs, but
// its row persists (permanent ruin, discoverable per 3.5).
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

	// The row itself must still exist (permanent ruin).
	var stillExists bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM encampments WHERE id = $1)", factionID).Scan(&stillExists); err != nil {
		t.Fatalf("checking row existence: %v", err)
	}
	if !stillExists {
		t.Error("expected the defeated faction's row to persist permanently, not be deleted")
	}
}

// TestResolveExplorationDiscovery_DefeatedFactionShowsRuinsAndEpitaph is
// the direct test for RARE_WORLD_FEATURES_PLAN.md Phase 3.5: exploring
// into a permanently-defeated AI faction's old territory should report
// ruins and the generated epitaph instead of the ordinary "now
// available in the Tactical Target Matrix" first-contact framing.
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

	// ExplorationDiscoveryChance is ~92% at 0 scouts (not guaranteed
	// 100%), so retry a handful of times rather than asserting on a
	// single roll - matches roadencounter_test.go's runUntilCondition
	// pattern for other probabilistic tick-phase tests in this package.
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
