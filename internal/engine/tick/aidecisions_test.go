package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func setAILevel(t *testing.T, db *sql.DB, campID string, level int) {
	t.Helper()
	if _, err := db.Exec("UPDATE encampments SET level = $1 WHERE id = $2", level, campID); err != nil {
		t.Fatalf("setting level: %v", err)
	}
}

func setGarrison(t *testing.T, db *sql.DB, campID string, soldiers, mechs int) {
	t.Helper()
	if _, err := db.Exec("UPDATE workshop_inventory SET soldiers = $1, mechs = $2 WHERE encampment_id = $3", soldiers, mechs, campID); err != nil {
		t.Fatalf("setting garrison: %v", err)
	}
}

func TestAIScoutDiscoversAnUndiscoveredRealPlayerNotAnotherAIFaction(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2001, "Faction Alpha", 0, 0, "TestRegion", true)
	seedEncampment(t, db, 2002, "Faction Beta", 1, 1, "TestRegion", true) // another AI, must never be scouted
	player := seedEncampment(t, db, 2003, "Player Camp", 2, 2, "TestRegion", false)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.aiScout(ctx, tx, faction, "TestRegion"); err != nil {
		t.Fatalf("aiScout: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var discoveredID string
	err = db.QueryRow("SELECT target_encampment_id FROM encampment_discoveries WHERE observer_encampment_id = $1", faction).Scan(&discoveredID)
	if err != nil {
		t.Fatalf("expected a discovery row, got error: %v", err)
	}
	if discoveredID != player {
		t.Errorf("expected the AI faction to discover the real player %s, discovered %s instead", player, discoveredID)
	}

	var decisionIntent string
	err = db.QueryRow("SELECT intent FROM ai_faction_decisions WHERE encampment_id = $1", faction).Scan(&decisionIntent)
	if err != nil {
		t.Fatalf("expected a decision log row: %v", err)
	}
	if decisionIntent != "scout" {
		t.Errorf("expected decision intent 'scout', got %q", decisionIntent)
	}
}

func TestPickFairAIRaidTargetExcludesTooWeakPlayersAndOtherAIFactions(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2004, "Faction Gamma", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 10)

	weakPlayer := seedEncampment(t, db, 2005, "Weak Player", 1, 0, "TestRegion", false)
	setAILevel(t, db, weakPlayer, 1) // 9 levels below - must be excluded
	fairPlayer := seedEncampment(t, db, 2006, "Fair Player", 2, 0, "TestRegion", false)
	setAILevel(t, db, fairPlayer, 9) // 1 level below - within the fairness band
	otherFaction := seedEncampment(t, db, 2007, "Faction Delta", 3, 0, "TestRegion", true)
	setAILevel(t, db, otherFaction, 9) // fair by level, but is an AI faction - must be excluded

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	for _, target := range []string{weakPlayer, fairPlayer, otherFaction} {
		if _, err := tx.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, target); err != nil {
			t.Fatalf("seeding discovery: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	target, err := e.pickFairAIRaidTarget(ctx, tx, faction, 10)
	if err != nil {
		t.Fatalf("pickFairAIRaidTarget: %v", err)
	}
	if target == nil {
		t.Fatal("expected a fair target to be found")
	}
	if target.id != fairPlayer {
		t.Errorf("expected the fair target %s (level 9), got %s", fairPlayer, target.id)
	}
}

func TestLaunchAIRaidCreatesAValidRaidAndDebitsGarrison(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2008, "Faction Epsilon", 0, 0, "TestRegion", true)
	setGarrison(t, db, faction, 100, 50)
	player := seedEncampment(t, db, 2009, "Target Player", 10, 0, "TestRegion", false)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	target, err := e.pickFairAIRaidTarget(ctx, tx, faction, 1)
	if err != nil {
		t.Fatalf("pickFairAIRaidTarget: %v", err)
	}
	if target == nil {
		// Not discovered yet - discover it directly for this test's purposes.
		if _, err := tx.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, player); err != nil {
			t.Fatalf("seeding discovery: %v", err)
		}
		target, err = e.pickFairAIRaidTarget(ctx, tx, faction, 1)
		if err != nil || target == nil {
			t.Fatalf("expected a fair target after seeding discovery, got %v, err %v", target, err)
		}
	}

	if err := e.launchAIRaid(ctx, tx, faction, *target); err != nil {
		t.Fatalf("launchAIRaid: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var raidID, state, movementState string
	var attackerID, defenderID string
	err = db.QueryRow("SELECT id, attacker_id, defender_id, state, movement_state FROM raids WHERE attacker_id = $1", faction).
		Scan(&raidID, &attackerID, &defenderID, &state, &movementState)
	if err != nil {
		t.Fatalf("expected a raid row: %v", err)
	}
	if defenderID != player {
		t.Errorf("expected defender_id %s, got %s", player, defenderID)
	}
	if state != "marching" || movementState != "moving" {
		t.Errorf("expected state='marching' movement_state='moving', got state=%q movement_state=%q", state, movementState)
	}

	var soldiersMobilized, mechsMobilized int
	err = db.QueryRow("SELECT soldiers_mobilized, mechs_mobilized FROM raid_forces WHERE raid_id = $1", raidID).Scan(&soldiersMobilized, &mechsMobilized)
	if err != nil {
		t.Fatalf("expected a raid_forces row: %v", err)
	}
	if soldiersMobilized <= 0 {
		t.Error("expected the AI faction to have committed some soldiers")
	}

	var remainingSoldiers, remainingMechs int
	_ = db.QueryRow("SELECT soldiers, mechs FROM workshop_inventory WHERE encampment_id = $1", faction).Scan(&remainingSoldiers, &remainingMechs)
	if remainingSoldiers+soldiersMobilized != 100 {
		t.Errorf("expected garrison debited by exactly the mobilized amount: 100 - %d should remain %d, got %d", soldiersMobilized, 100-soldiersMobilized, remainingSoldiers)
	}
	if remainingMechs+mechsMobilized != 50 {
		t.Errorf("expected mech garrison debited by exactly the mobilized amount: got %d remaining with %d mobilized", remainingMechs, mechsMobilized)
	}

	// A faction never strips its garrison to zero.
	if remainingSoldiers <= 0 {
		t.Error("expected some soldiers to remain at home after a raid launch")
	}
}

// TestAIFactionNeverRaidsAnotherAIFaction is the probabilistic-absence
// check for the raid path, mirroring
// TestEvaluateRoadBaseEncountersExcludesAIFactionBases's pattern from
// roadencounter_test.go: run the full decision loop many times and assert
// zero AI-vs-AI raids ever appear, not just check once.
func TestAIFactionNeverRaidsAnotherAIFaction(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionA := seedEncampment(t, db, 2010, "Faction Zeta", 0, 0, "TestRegion", true)
	setAILevel(t, db, factionA, 5)
	setGarrison(t, db, factionA, 200, 100)
	factionB := seedEncampment(t, db, 2011, "Faction Eta", 1, 0, "TestRegion", true)
	setAILevel(t, db, factionB, 5)

	// Force-discover factionB from factionA's perspective, as if a bug
	// let it happen - the exclusion must hold even if a discovery row
	// exists, not just because scouting never targets an AI faction.
	if _, err := db.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", factionA, factionB); err != nil {
		t.Fatalf("seeding discovery: %v", err)
	}

	for i := 0; i < 40; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		target, err := e.pickFairAIRaidTarget(ctx, tx, factionA, 5)
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("pickFairAIRaidTarget: %v", err)
		}
		if target != nil {
			if err := e.launchAIRaid(ctx, tx, factionA, *target); err != nil {
				_ = tx.Rollback()
				t.Fatalf("launchAIRaid: %v", err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM raids WHERE attacker_id = $1 AND defender_id = $2", factionA, factionB).Scan(&count)
	if count != 0 {
		t.Errorf("expected zero AI-vs-AI raids, got %d", count)
	}
}

// TestAILaunchedRaidParticipatesInRoadEncounters is the exit-criteria
// test AI_FACTION_DECISION_LOOP_PLAN.md calls "the single most important
// test in this plan" - it proves an AI-launched raid is a real column
// through the Phase 3/4 road-encounter system, not just a row that
// resolves in isolation.
func TestAILaunchedRaidParticipatesInRoadEncounters(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2012, "Faction Theta", 0, 0, "TestRegion", true)
	setGarrison(t, db, faction, 100, 50)
	player := seedEncampment(t, db, 2013, "Ambush Target Player", 10, 0, "TestRegion", false)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if _, err := tx.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, player); err != nil {
		t.Fatalf("seeding discovery: %v", err)
	}
	target, err := e.pickFairAIRaidTarget(ctx, tx, faction, 1)
	if err != nil || target == nil {
		t.Fatalf("expected a fair target, got %v, err %v", target, err)
	}
	if err := e.launchAIRaid(ctx, tx, faction, *target); err != nil {
		t.Fatalf("launchAIRaid: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var aiRaidID string
	if err := db.QueryRow("SELECT id FROM raids WHERE attacker_id = $1", faction).Scan(&aiRaidID); err != nil {
		t.Fatalf("expected the AI raid to exist: %v", err)
	}
	// Force it to the midpoint of its route, same trick seedMarchingRaid
	// uses, so it's guaranteed to be in encounter range of a human column
	// converging on the same point.
	if _, err := db.Exec(`
		UPDATE raids SET leg_started_at = $1, leg_total_minutes = 20
		WHERE id = $2`, time.Now().UTC().Add(-10*time.Minute), aiRaidID); err != nil {
		t.Fatalf("repositioning AI raid: %v", err)
	}
	var destX, destY int
	_ = db.QueryRow("SELECT destination_x, destination_y FROM raids WHERE id = $1", aiRaidID).Scan(&destX, &destY)
	// AI raid marches from (0,0) toward the player at (10,0); a human
	// column marching the reverse direction converges at the midpoint.
	humanAttacker := seedEncampment(t, db, 2014, "Human Column Owner", destX, destY, "TestRegion", false)
	seedMarchingRaid(t, db, humanAttacker, destX, destY, 0, 0, "TestRegion")

	runUntilCondition(t, db,
		func(tx *sql.Tx) error { return e.evaluateRoadEncounters(context.Background(), tx) },
		func() bool {
			var count int
			_ = db.QueryRow("SELECT COUNT(*) FROM road_encounters WHERE (raid_a_id = $1 OR raid_b_id = $1) AND status = 'pending'", aiRaidID).Scan(&count)
			return count > 0
		})

	var movementState string
	if err := db.QueryRow("SELECT movement_state FROM raids WHERE id = $1", aiRaidID).Scan(&movementState); err != nil {
		t.Fatalf("querying AI raid state: %v", err)
	}
	if movementState != "encounter_pending" {
		t.Errorf("expected the AI-launched raid to be frozen by a road encounter like a human raid, got movement_state %q", movementState)
	}
}
