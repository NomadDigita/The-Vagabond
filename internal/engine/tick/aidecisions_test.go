package tick

import (
	"context"
	"database/sql"
	"math/rand"
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

// TestAIScoutCanDiscoverAnotherAIFaction verifies aiScout no longer
// excludes AI factions as scouting targets - AI-vs-AI conflict is now in
// scope (AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2, superseding
// AI_FACTION_DECISION_LOOP_PLAN.md's "deliberately out of scope" note).
// Faction Beta is seeded first (closer/earlier established), so the
// existing danger/established_at ordering picks it before the player.
// TestMaybeAIFlee_TriggersWhenGarrisonIsDesperatelyLow verifies the AI
// decision loop's use of Ghost Protocol (section 3.4): a faction with a
// near-zero garrison relocates, pays the proportional cost, and clears
// any known_locations rows targeting it.
func TestMaybeAIFlee_TriggersWhenGarrisonIsDesperatelyLow(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2101, "Faction Desperate", 0, 0, "TestRegion", true)
	if _, err := db.Exec("UPDATE workshop_inventory SET soldiers = 2, mechs = 1 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding low garrison: %v", err)
	}
	if _, err := db.Exec("UPDATE resources SET scrap = 100, metal = 200, crystal = 50, dollars = 400 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}
	var originalCoordID string
	if err := db.QueryRow("SELECT coordinate_id FROM encampments WHERE id = $1", faction).Scan(&originalCoordID); err != nil {
		t.Fatalf("reading original coordinate: %v", err)
	}

	observer := seedEncampment(t, db, 2102, "Observer Faction", 1, 1, "TestRegion", true)
	if _, err := db.Exec("INSERT INTO known_locations (observer_encampment_id, target_encampment_id, x, y, region) VALUES ($1, $2, 0, 0, 'TestRegion')", observer, faction); err != nil {
		t.Fatalf("seeding known_locations: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	fled, err := e.maybeAIFlee(ctx, tx, faction)
	if err != nil {
		t.Fatalf("maybeAIFlee: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if !fled {
		t.Fatal("expected the faction to flee given a garrison of only 3 total units")
	}

	var newCoordID string
	_ = db.QueryRow("SELECT coordinate_id FROM encampments WHERE id = $1", faction).Scan(&newCoordID)
	if newCoordID == originalCoordID {
		t.Error("expected the faction to have relocated")
	}

	var scrap float64
	_ = db.QueryRow("SELECT scrap FROM resources WHERE encampment_id = $1", faction).Scan(&scrap)
	if scrap != 50 {
		t.Errorf("expected 50%% of scrap (100 -> 50) to be deducted, got %v", scrap)
	}

	var knownLocationsCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM known_locations WHERE target_encampment_id = $1", faction).Scan(&knownLocationsCount)
	if knownLocationsCount != 0 {
		t.Error("expected known_locations targeting the fleeing faction to be cleared")
	}

	var decisionIntent string
	_ = db.QueryRow("SELECT intent FROM ai_faction_decisions WHERE encampment_id = $1 ORDER BY decided_at DESC LIMIT 1", faction).Scan(&decisionIntent)
	if decisionIntent != "flee" {
		t.Errorf("expected the logged decision intent to be 'flee', got %q", decisionIntent)
	}
}

// TestMaybeAIFlee_DoesNothingWithAHealthyGarrison is the negative case:
// a faction well above the threshold never flees.
func TestMaybeAIFlee_DoesNothingWithAHealthyGarrison(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2103, "Faction Healthy", 0, 0, "TestRegion", true)
	if _, err := db.Exec("UPDATE workshop_inventory SET soldiers = 500, mechs = 100 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding healthy garrison: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	fled, err := e.maybeAIFlee(ctx, tx, faction)
	if err != nil {
		t.Fatalf("maybeAIFlee: %v", err)
	}
	if fled {
		t.Error("expected a healthy-garrison faction to never flee")
	}
}

// TestAIScoutCanDiscoverAnotherAIFaction verifies aiScout no longer
// excludes AI factions as scouting targets - AI-vs-AI conflict is now in
// scope (AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2, superseding
// AI_FACTION_DECISION_LOOP_PLAN.md's "deliberately out of scope" note).
// Faction Beta is seeded first (closer/earlier established), so the
// existing danger/established_at ordering picks it before the player.
func TestAIScoutCanDiscoverAnotherAIFaction(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2001, "Faction Alpha", 0, 0, "TestRegion", true)
	otherFaction := seedEncampment(t, db, 2002, "Faction Beta", 1, 1, "TestRegion", true)
	seedEncampment(t, db, 2003, "Player Camp", 2, 2, "TestRegion", false)

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
	if discoveredID != otherFaction {
		t.Errorf("expected the AI faction to be able to discover another AI faction %s, discovered %s instead", otherFaction, discoveredID)
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

// TestPickFairAIRaidTargetOnlyAllowsAttackingAtOrAboveOwnLevel verifies
// the 2026-08-01 fairness inversion: a faction may only raid a target at
// its own level or above, never below - "lower level can attack higher
// level but higher level can't attack lower level," per the project
// owner. Runs pickFairAIRaidTarget many times since it randomizes
// between the normal and wide band on each call; a below-level target
// must never appear in any of them, and an at-or-above-level target must
// appear at least once.
func TestPickFairAIRaidTargetOnlyAllowsAttackingAtOrAboveOwnLevel(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2004, "Faction Gamma", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 10)

	farBelowPlayer := seedEncampment(t, db, 2005, "Weak Player", 1, 0, "TestRegion", false)
	setAILevel(t, db, farBelowPlayer, 1) // far below - must always be excluded

	oneBelowFaction := seedEncampment(t, db, 2007, "Faction Delta", 3, 0, "TestRegion", true)
	setAILevel(t, db, oneBelowFaction, 9) // 1 level BELOW - the exact case the project owner called out
	// as the one that "should almost never" happen anymore: must be excluded now, even though the old
	// (pre-2026-08-01) rule would have allowed it.

	twoAboveFaction := seedEncampment(t, db, 2006, "Faction Fair", 2, 0, "TestRegion", true)
	setAILevel(t, db, twoAboveFaction, 12) // 2 levels ABOVE, inside the normal band - a valid "punch up" target.

	sameLevelFaction := seedEncampment(t, db, 2015, "Faction Even", 4, 0, "TestRegion", true)
	setAILevel(t, db, sameLevelFaction, 10) // exactly level - allowed, "at or above" includes equal.

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	for _, target := range []string{farBelowPlayer, oneBelowFaction, twoAboveFaction, sameLevelFaction} {
		if _, err := tx.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, target); err != nil {
			t.Fatalf("seeding discovery: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	seenAboveOrSame := false
	for i := 0; i < 50; i++ {
		tx2, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		target, err := e.pickFairAIRaidTarget(ctx, tx2, faction, 10, sql.NullString{})
		_ = tx2.Rollback()
		if err != nil {
			t.Fatalf("pickFairAIRaidTarget: %v", err)
		}
		if target == nil {
			continue
		}
		if target.id == farBelowPlayer || target.id == oneBelowFaction {
			t.Fatalf("pickFairAIRaidTarget returned a below-own-level target %q - attack-up-only rule violated", target.name)
		}
		if target.id == twoAboveFaction || target.id == sameLevelFaction {
			seenAboveOrSame = true
		}
	}
	if !seenAboveOrSame {
		t.Error("expected pickFairAIRaidTarget to sometimes return an at-or-above-level target across 50 attempts")
	}
}

// TestPickOverdueRaidTargetGuaranteesEligibilityRegardlessOfProbability
// verifies the second half of the 2026-08-01 direction: a discovered,
// level-4+ real player who hasn't been raided in aiOverdueRaidThreshold
// is returned by pickOverdueRaidTarget - the guarantee mechanism that
// bypasses the probability roll entirely - while a target raided
// recently, a target below the level floor, and another AI faction (not
// eligible for this specific guarantee) are all correctly excluded.
func TestPickOverdueRaidTargetGuaranteesEligibilityRegardlessOfProbability(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2016, "Faction Iota", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 3)

	overdueTarget := seedEncampment(t, db, 2017, "Overdue Player", 1, 0, "TestRegion", false)
	setAILevel(t, db, overdueTarget, 12) // well above the level-4 floor, never raided - should be guaranteed-eligible

	tooLowLevel := seedEncampment(t, db, 2018, "New Player", 2, 0, "TestRegion", false)
	setAILevel(t, db, tooLowLevel, 3) // below aiOverdueMinTargetLevel (4) - must be excluded from the guarantee

	recentlyRaidedTarget := seedEncampment(t, db, 2019, "Recently Raided Player", 3, 0, "TestRegion", false)
	setAILevel(t, db, recentlyRaidedTarget, 12)

	anotherAIFaction := seedEncampment(t, db, 2020, "Faction Kappa", 4, 0, "TestRegion", true)
	setAILevel(t, db, anotherAIFaction, 12) // AI-vs-AI is deliberately excluded from this guarantee

	for _, target := range []string{overdueTarget, tooLowLevel, recentlyRaidedTarget, anotherAIFaction} {
		if _, err := db.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, target); err != nil {
			t.Fatalf("seeding discovery: %v", err)
		}
	}
	// recentlyRaidedTarget was raided 1 hour ago - well inside the 10-hour threshold.
	if _, err := db.Exec(`
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time, created_at)
		VALUES ($1, $2, 'resolved', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP - INTERVAL '1 hour')`,
		faction, recentlyRaidedTarget); err != nil {
		t.Fatalf("seeding a recent raid: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	target, err := e.pickOverdueRaidTarget(ctx, tx, faction, 3, sql.NullString{})
	if err != nil {
		t.Fatalf("pickOverdueRaidTarget: %v", err)
	}
	if target == nil {
		t.Fatal("expected an overdue target to be found")
	}
	if target.id != overdueTarget {
		t.Errorf("expected the overdue target %s, got %s", overdueTarget, target.id)
	}
}

// TestDecideOneAIFactionPrioritizesOverdueGuaranteeOverProbabilityRoll
// verifies decideOneAIFaction actually raids an overdue-eligible target
// on the very next decision, with no dependency on the probability roll
// - the "no roll, no exception" guarantee.
func TestDecideOneAIFactionPrioritizesOverdueGuaranteeOverProbabilityRoll(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2021, "Faction Lambda", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 3)
	setGarrison(t, db, faction, 100, 50)

	overdueTarget := seedEncampment(t, db, 2022, "Guaranteed Target", 1, 0, "TestRegion", false)
	setAILevel(t, db, overdueTarget, 12)
	if _, err := db.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, overdueTarget); err != nil {
		t.Fatalf("seeding discovery: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.decideOneAIFaction(ctx, tx, faction, 3, "TestRegion"); err != nil {
		t.Fatalf("decideOneAIFaction: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var raidCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM raids WHERE attacker_id = $1 AND defender_id = $2", faction, overdueTarget).Scan(&raidCount)
	if raidCount != 1 {
		t.Errorf("expected exactly one guaranteed raid against the overdue target, got %d", raidCount)
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
	target, err := e.pickFairAIRaidTarget(ctx, tx, faction, 1, sql.NullString{})
	if err != nil {
		t.Fatalf("pickFairAIRaidTarget: %v", err)
	}
	if target == nil {
		// Not discovered yet - discover it directly for this test's purposes.
		if _, err := tx.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, player); err != nil {
			t.Fatalf("seeding discovery: %v", err)
		}
		target, err = e.pickFairAIRaidTarget(ctx, tx, faction, 1, sql.NullString{})
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

// TestAIFactionCanRaidAnotherAIFactionRarely mirrors
// TestEvaluateRoadBaseEncountersExcludesAIFactionBases's probabilistic
// pattern from roadencounter_test.go: AI-vs-AI conflict is in scope
// (AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2), and running many
// iterations at aiVsAIRaidProbabilityWhenEligible should produce at least
// one AI-vs-AI raid (comfortably so, now that the probability was raised
// 2026-08-01 toward a twice-daily real-world cadence rather than staying
// rare background texture), and it should never generate a public
// world_news headline.
func TestAIFactionCanRaidAnotherAIFactionRarely(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	factionA := seedEncampment(t, db, 2010, "Faction Zeta", 0, 0, "TestRegion", true)
	setAILevel(t, db, factionA, 5)
	setGarrison(t, db, factionA, 200, 100)
	factionB := seedEncampment(t, db, 2011, "Faction Eta", 1, 0, "TestRegion", true)
	setAILevel(t, db, factionB, 5)

	if _, err := db.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", factionA, factionB); err != nil {
		t.Fatalf("seeding discovery: %v", err)
	}

	for i := 0; i < 100; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		target, err := e.pickFairAIRaidTarget(ctx, tx, factionA, 5, sql.NullString{})
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("pickFairAIRaidTarget: %v", err)
		}
		if target != nil && target.isAIFaction && rand.Float64() < aiVsAIRaidProbabilityWhenEligible {
			if err := e.launchAIRaid(ctx, tx, factionA, *target); err != nil {
				_ = tx.Rollback()
				t.Fatalf("launchAIRaid: %v", err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		// Garrison is finite and each raid debits it - replenish so later
		// iterations can still commit forces even after earlier raids fired.
		setGarrison(t, db, factionA, 200, 100)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM raids WHERE attacker_id = $1 AND defender_id = $2", factionA, factionB).Scan(&count)
	if count == 0 {
		t.Errorf("expected at least one AI-vs-AI raid across 100 iterations at aiVsAIRaidProbabilityWhenEligible (%.0f%%), got zero", aiVsAIRaidProbabilityWhenEligible*100)
	}

	var newsCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM world_news WHERE headline LIKE '%Faction Eta%'").Scan(&newsCount)
	if newsCount != 0 {
		t.Errorf("expected AI-vs-AI raids on Faction Eta to never generate a public world_news headline, got %d", newsCount)
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
	target, err := e.pickFairAIRaidTarget(ctx, tx, faction, 1, sql.NullString{})
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
