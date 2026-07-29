package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func seedScoutMission(t *testing.T, db *sql.DB, encampmentID string, phase string, scoutsCommitted int) string {
	t.Helper()
	var id string
	err := db.QueryRow(`
		INSERT INTO scout_missions (encampment_id, scouts_committed, phase)
		VALUES ($1, $2, $3) RETURNING id`, encampmentID, scoutsCommitted, phase).Scan(&id)
	if err != nil {
		t.Fatalf("seeding scout mission: %v", err)
	}
	return id
}

// TestScoutMissionFindsTarget_LocksLocationAndTransitionsToReturning
// verifies the core discovery mechanic from section 3.1-3.3: an
// undiscovered target gets a permanent encampment_discoveries row AND a
// known_locations coordinate snapshot, the mission transitions to
// 'returning' with a computed ETA, and both parties are notified
// appropriately (the scout's owner gets a non-mutable "general" contact
// alert - not "route_status": a discovery event must never be
// mutable/muteable, per notifications/preferences.go's own contract -
// the found party gets a non-mutable "general" heads-up too).
// TestScoutMissionFindsTarget_ContactAlertSurvivesRouteStatusMute is a
// regression test for a real reported bug: the CONTACT!/discovery
// notification was originally (incorrectly) tagged "route_status", a
// mutable category. A player who'd muted routine route pings (the
// "still searching" chatter) would then also silently lose the one
// notification that actually mattered - their scout finding a base.
// Discovery alerts must never be mutable, per notifications/
// preferences.go's own MutableCategories contract.
func TestScoutMissionFindsTarget_ContactAlertSurvivesRouteStatusMute(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	scout := seedEncampment(t, db, 4007, "Muted Scout Origin", 0, 0, "TestRegion", false)
	seedEncampment(t, db, 4008, "Muted Distant Target", 50, 0, "OtherRegion", false)
	if _, err := db.Exec(`
		INSERT INTO notification_preferences (user_id, mute_route_status) VALUES ($1, TRUE)
		ON CONFLICT (user_id) DO UPDATE SET mute_route_status = TRUE`, int64(4007)); err != nil {
		t.Fatalf("seeding muted preference: %v", err)
	}

	missionID := seedScoutMission(t, db, scout, "searching", 5)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	m := searchingScoutMission{id: missionID, encampmentID: scout, scoutsCommitted: 5, userID: 4007, originX: 0, originY: 0}
	if err := e.scoutMissionFindsTarget(ctx, tx, m); err != nil {
		t.Fatalf("scoutMissionFindsTarget: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var notifiedCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message LIKE '%CONTACT%'", int64(4007)).Scan(&notifiedCount)
	if notifiedCount == 0 {
		t.Error("expected the CONTACT! discovery notification to reach the player even with route_status muted")
	}
}

func TestScoutMissionFindsTarget_LocksLocationAndTransitionsToReturning(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	scout := seedEncampment(t, db, 4001, "Scout Origin", 0, 0, "TestRegion", false)
	target := seedEncampment(t, db, 4002, "Distant Target", 50, 0, "OtherRegion", false)

	missionID := seedScoutMission(t, db, scout, "searching", 5)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	m := searchingScoutMission{id: missionID, encampmentID: scout, scoutsCommitted: 5, userID: 4001, originX: 0, originY: 0}
	if err := e.scoutMissionFindsTarget(ctx, tx, m); err != nil {
		t.Fatalf("scoutMissionFindsTarget: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var discoveryCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampment_discoveries WHERE observer_encampment_id = $1 AND target_encampment_id = $2", scout, target).Scan(&discoveryCount)
	if discoveryCount != 1 {
		t.Errorf("expected exactly one encampment_discoveries row, got %d", discoveryCount)
	}

	var lockedX, lockedY int
	if err := db.QueryRow("SELECT x, y FROM known_locations WHERE observer_encampment_id = $1 AND target_encampment_id = $2", scout, target).Scan(&lockedX, &lockedY); err != nil {
		t.Fatalf("expected a known_locations row: %v", err)
	}
	if lockedX != 50 || lockedY != 0 {
		t.Errorf("expected the locked coordinate to match the target's position (50,0), got (%d,%d)", lockedX, lockedY)
	}

	var phase string
	var returnMinutes float64
	if err := db.QueryRow("SELECT phase, return_leg_total_minutes FROM scout_missions WHERE id = $1", missionID).Scan(&phase, &returnMinutes); err != nil {
		t.Fatalf("reading mission: %v", err)
	}
	if phase != "returning" {
		t.Errorf("expected phase 'returning', got %q", phase)
	}
	if returnMinutes != 500 { // Manhattan distance 50 * 10 minutes/tile
		t.Errorf("expected a 500-minute return leg (50 tiles * 10 min), got %v", returnMinutes)
	}

	var scoutNotified, targetNotified int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND category = 'general'", int64(4001)).Scan(&scoutNotified)
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND category = 'general'", int64(4002)).Scan(&targetNotified)
	if scoutNotified == 0 {
		t.Error("expected the scouting player to receive a non-mutable 'general' contact notification (a discovery alert must never be muteable)")
	}
	if targetNotified == 0 {
		t.Error("expected the discovered player to receive a non-mutable 'general' heads-up")
	}
}

// TestScoutMissionFindsTarget_NoTargetsLeftStaysSearching covers the
// edge case where an observer has already discovered everyone in the
// world - it should keep searching rather than error.
func TestScoutMissionFindsTarget_NoTargetsLeftStaysSearching(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	scout := seedEncampment(t, db, 4003, "Lonely Scout", 0, 0, "TestRegion", false)
	missionID := seedScoutMission(t, db, scout, "searching", 3)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	m := searchingScoutMission{id: missionID, encampmentID: scout, scoutsCommitted: 3, userID: 4003, originX: 0, originY: 0}
	if err := e.scoutMissionFindsTarget(ctx, tx, m); err != nil {
		t.Fatalf("scoutMissionFindsTarget: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var phase string
	_ = db.QueryRow("SELECT phase FROM scout_missions WHERE id = $1", missionID).Scan(&phase)
	if phase != "searching" {
		t.Errorf("expected the mission to remain in 'searching' when nothing is left to find, got %q", phase)
	}
}

// TestCompleteReturnedScoutMissions_PaysResourcesAndReturnsScouts verifies
// section 3.6/3.7: resources are credited proportional to scouts and
// mission duration, the committed scouts return to workshop_inventory,
// and the mission is marked complete.
func TestCompleteReturnedScoutMissions_PaysResourcesAndReturnsScouts(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	scout := seedEncampment(t, db, 4004, "Returning Scout Camp", 0, 0, "TestRegion", false)
	if _, err := db.Exec("UPDATE workshop_inventory SET scouts = 0 WHERE encampment_id = $1", scout); err != nil {
		t.Fatalf("zeroing scouts: %v", err)
	}
	if _, err := db.Exec("UPDATE resources SET scrap = 0, metal = 0 WHERE encampment_id = $1", scout); err != nil {
		t.Fatalf("zeroing resources: %v", err)
	}

	startedAt := time.Now().UTC().Add(-2 * time.Hour) // a 2-hour mission
	var missionID string
	err := db.QueryRow(`
		INSERT INTO scout_missions (encampment_id, scouts_committed, phase, started_at, return_eta)
		VALUES ($1, 5, 'returning', $2, $3) RETURNING id`,
		scout, startedAt, time.Now().UTC().Add(-time.Minute)).Scan(&missionID)
	if err != nil {
		t.Fatalf("seeding returning mission: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.completeReturnedScoutMissions(ctx, tx); err != nil {
		t.Fatalf("completeReturnedScoutMissions: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var scrap, metal float64
	if err := db.QueryRow("SELECT scrap, metal FROM resources WHERE encampment_id = $1", scout).Scan(&scrap, &metal); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	// 50/hour/scout * 2 hours * 5 scouts = 500 total value, 60/40 split.
	if scrap < 250 || scrap > 350 {
		t.Errorf("expected roughly 300 scrap credited (60%% of ~500 total value), got %v", scrap)
	}
	if metal < 150 || metal > 250 {
		t.Errorf("expected roughly 200 metal credited (40%% of ~500 total value), got %v", metal)
	}

	var scoutsBack int
	_ = db.QueryRow("SELECT scouts FROM workshop_inventory WHERE encampment_id = $1", scout).Scan(&scoutsBack)
	if scoutsBack != 5 {
		t.Errorf("expected all 5 committed scouts to return, got %d", scoutsBack)
	}

	var phase string
	_ = db.QueryRow("SELECT phase FROM scout_missions WHERE id = $1", missionID).Scan(&phase)
	if phase != "complete" {
		t.Errorf("expected phase 'complete', got %q", phase)
	}

	var notified int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND category = 'route_status'", int64(4004)).Scan(&notified)
	if notified == 0 {
		t.Error("expected a route_status completion notification")
	}
}

// TestPingInTransitScoutMissions_SendsETAAndRespectsGate verifies the
// periodic en-route ping only fires when the cadence gate is open.
func TestPingInTransitScoutMissions_SendsETAAndRespectsGate(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	scout := seedEncampment(t, db, 4005, "En Route Scout Camp", 0, 0, "TestRegion", false)
	target := seedEncampment(t, db, 4006, "Found Target Camp", 10, 0, "TestRegion", false)

	// Gate open (never notified) - should ping.
	_, err := db.Exec(`
		INSERT INTO scout_missions (encampment_id, scouts_committed, phase, found_target_encampment_id,
			found_x, found_y, found_region, origin_x, origin_y, return_leg_started_at, return_leg_total_minutes, return_eta)
		VALUES ($1, 3, 'returning', $2, 10, 0, 'TestRegion', 0, 0, $3, 100, $4)`,
		scout, target, time.Now().UTC().Add(-10*time.Minute), time.Now().UTC().Add(50*time.Minute))
	if err != nil {
		t.Fatalf("seeding returning mission: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.pingInTransitScoutMissions(ctx, tx); err != nil {
		t.Fatalf("pingInTransitScoutMissions: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var notifiedCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message LIKE '%en route home%'", int64(4005)).Scan(&notifiedCount)
	if notifiedCount == 0 {
		t.Error("expected an en-route ping when the cadence gate was open")
	}

	// Gate now closed (just notified) - a second immediate call must not
	// double-ping.
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.pingInTransitScoutMissions(ctx, tx); err != nil {
		t.Fatalf("pingInTransitScoutMissions (second call): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var secondCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message LIKE '%en route home%'", int64(4005)).Scan(&secondCount)
	if secondCount != notifiedCount {
		t.Errorf("expected no additional ping while the cadence gate is still closed, got %d (was %d)", secondCount, notifiedCount)
	}
}
