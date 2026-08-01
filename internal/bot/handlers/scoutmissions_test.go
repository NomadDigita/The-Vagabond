package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func seedScoutDispatchCamp(t *testing.T, telegramID int64, x, y int, availableScouts int) string {
	t.Helper()
	db := rankingTestDB(t)
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $2, 'plains', 'TestRegion', 'flat')
		RETURNING id`, x, y).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id) VALUES ($1, 'Scout Dispatch Camp', $2) RETURNING id`,
		telegramID, coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO workshop_inventory (encampment_id, scouts) VALUES ($1, $2)", campID, availableScouts); err != nil {
		t.Fatalf("seeding workshop_inventory: %v", err)
	}
	return campID
}

// TestDoDispatchScoutMission_CommitsScoutsAndCreatesMission verifies the
// core dispatch path: scouts are debited from workshop_inventory and a
// new scout_missions row is created in the 'searching' phase.
func TestDoDispatchScoutMission_CommitsScoutsAndCreatesMission(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedScoutDispatchCamp(t, 9601, 600, 600, 10)

	h := &ScoutMissionsHandler{DB: db}
	msg, err := h.doDispatchScoutMission(ctx, camp, 5)
	if err != nil {
		t.Fatalf("doDispatchScoutMission: %v (%s)", err, msg)
	}
	if !strings.Contains(msg, "SCOUT PARTY DISPATCHED") {
		t.Errorf("expected a dispatch confirmation message, got %q", msg)
	}

	var remainingScouts int
	if err := db.QueryRow("SELECT scouts FROM workshop_inventory WHERE encampment_id = $1", camp).Scan(&remainingScouts); err != nil {
		t.Fatalf("reading remaining scouts: %v", err)
	}
	if remainingScouts != 5 {
		t.Errorf("expected 5 scouts remaining (10 - 5 committed), got %d", remainingScouts)
	}

	var phase string
	var scoutsCommitted int
	if err := db.QueryRow("SELECT phase, scouts_committed FROM scout_missions WHERE encampment_id = $1", camp).Scan(&phase, &scoutsCommitted); err != nil {
		t.Fatalf("reading mission: %v", err)
	}
	if phase != "searching" || scoutsCommitted != 5 {
		t.Errorf("expected a 'searching' mission with 5 scouts committed, got phase=%q scouts=%d", phase, scoutsCommitted)
	}
}

// TestDoDispatchScoutMission_RejectsInsufficientScouts verifies the
// validation path when the outpost doesn't have enough available scouts.
func TestDoDispatchScoutMission_RejectsInsufficientScouts(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedScoutDispatchCamp(t, 9602, 601, 601, 2)

	h := &ScoutMissionsHandler{DB: db}
	if _, err := h.doDispatchScoutMission(ctx, camp, 5); err == nil {
		t.Error("expected an error when requesting more scouts than available")
	}

	var missionCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM scout_missions WHERE encampment_id = $1", camp).Scan(&missionCount)
	if missionCount != 0 {
		t.Error("expected no scout_missions row to be created on a rejected dispatch")
	}
}

// TestDoDispatchScoutMission_AllowsUpToCapThenRejects verifies the
// multi-mission cap: up to maxConcurrentScoutMissions (3) concurrent
// dispatches succeed, and the next one is rejected while all 3 slots are
// still active.
func TestDoDispatchScoutMission_AllowsUpToCapThenRejects(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedScoutDispatchCamp(t, 9603, 602, 602, 20)

	h := &ScoutMissionsHandler{DB: db}
	for i := 0; i < maxConcurrentScoutMissions; i++ {
		if _, err := h.doDispatchScoutMission(ctx, camp, 2); err != nil {
			t.Fatalf("dispatch %d (of %d allowed) should succeed: %v", i+1, maxConcurrentScoutMissions, err)
		}
	}
	if _, err := h.doDispatchScoutMission(ctx, camp, 2); err == nil {
		t.Errorf("expected dispatch beyond the cap of %d to be rejected", maxConcurrentScoutMissions)
	}

	var missionCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM scout_missions WHERE encampment_id = $1 AND phase IN ('searching', 'returning')", camp).Scan(&missionCount); err != nil {
		t.Fatalf("counting active missions: %v", err)
	}
	if missionCount != maxConcurrentScoutMissions {
		t.Errorf("expected exactly %d active missions, got %d", maxConcurrentScoutMissions, missionCount)
	}
}

// TestRenderScoutStatus_ReflectsEachPhase verifies the beautified status
// text correctly reports each phase, and the active-mission count the
// panel uses to decide whether a dispatch slot is still free.
func TestRenderScoutStatus_ReflectsEachPhase(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedScoutDispatchCamp(t, 9604, 603, 603, 10)
	h := &ScoutMissionsHandler{DB: db}

	// No mission yet.
	text, active, err := h.renderScoutStatus(ctx, camp)
	if err != nil {
		t.Fatalf("renderScoutStatus: %v", err)
	}
	if active != 0 {
		t.Errorf("expected active=0 with no mission dispatched, got %d", active)
	}
	if !strings.Contains(text, "No scout party is currently out") {
		t.Errorf("expected a 'no scout party' message, got %q", text)
	}

	// Searching phase.
	if _, err := h.doDispatchScoutMission(ctx, camp, 3); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	text, active, err = h.renderScoutStatus(ctx, camp)
	if err != nil {
		t.Fatalf("renderScoutStatus: %v", err)
	}
	if active != 1 {
		t.Errorf("expected active=1 while one party is searching, got %d", active)
	}
	if !strings.Contains(text, "still searching") {
		t.Errorf("expected searching-phase text, got %q", text)
	}
	if !strings.Contains(text, "Party 1/3") {
		t.Errorf("expected the first mission to be labeled 'Party 1/3', got %q", text)
	}

	// Returning phase.
	if _, err := db.Exec("UPDATE scout_missions SET phase = 'returning' WHERE encampment_id = $1", camp); err != nil {
		t.Fatalf("forcing returning phase: %v", err)
	}
	text, active, err = h.renderScoutStatus(ctx, camp)
	if err != nil {
		t.Fatalf("renderScoutStatus: %v", err)
	}
	if active != 1 {
		t.Errorf("expected active=1 while returning, got %d", active)
	}
	if !strings.Contains(text, "returning home") {
		t.Errorf("expected returning-phase text, got %q", text)
	}
}

// TestRenderScoutStatus_ListsMultipleConcurrentMissionsIndependently
// verifies the core of the multi-scouting feature: three missions dispatched
// to the same encampment are all listed, each with its own Party N/3 label
// and its own phase, rather than only the most recently dispatched one.
func TestRenderScoutStatus_ListsMultipleConcurrentMissionsIndependently(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedScoutDispatchCamp(t, 9605, 604, 604, 20)
	h := &ScoutMissionsHandler{DB: db}

	for i := 0; i < maxConcurrentScoutMissions; i++ {
		if _, err := h.doDispatchScoutMission(ctx, camp, 2); err != nil {
			t.Fatalf("dispatch %d: %v", i+1, err)
		}
	}

	// Put the first-dispatched mission into 'returning' so the two phases
	// are both represented, and confirm both survive independently in the
	// listing (i.e. neither overwrites the other, which an ORDER BY...
	// LIMIT 1 query would have done before this feature).
	if _, err := db.Exec(`
		UPDATE scout_missions SET phase = 'returning'
		WHERE id = (SELECT id FROM scout_missions WHERE encampment_id = $1 ORDER BY started_at ASC LIMIT 1)`, camp); err != nil {
		t.Fatalf("forcing first mission to returning: %v", err)
	}

	text, active, err := h.renderScoutStatus(ctx, camp)
	if err != nil {
		t.Fatalf("renderScoutStatus: %v", err)
	}
	if active != maxConcurrentScoutMissions {
		t.Errorf("expected active=%d with all slots committed, got %d", maxConcurrentScoutMissions, active)
	}
	for i := 1; i <= maxConcurrentScoutMissions; i++ {
		label := fmt.Sprintf("Party %d/%d", i, maxConcurrentScoutMissions)
		if !strings.Contains(text, label) {
			t.Errorf("expected status text to contain %q, got %q", label, text)
		}
	}
	if !strings.Contains(text, "returning home") {
		t.Errorf("expected the first mission's 'returning home' text to be present, got %q", text)
	}
	if !strings.Contains(text, "still searching") {
		t.Errorf("expected the remaining missions' 'still searching' text to be present, got %q", text)
	}
	if strings.Contains(text, "slots free") {
		t.Errorf("expected no 'slots free' hint once all %d slots are committed, got %q", maxConcurrentScoutMissions, text)
	}
}

// TestScoutMissionsHandler_Construction is a minimal smoke test - the
// handler's telebot.Context-dependent paths are covered end-to-end in
// internal/engine/tick/scoutmissions_test.go and via the pure functions
// above instead.
func TestScoutMissionsHandler_Construction(t *testing.T) {
	db := rankingTestDB(t)
	h := NewScoutMissionsHandler(db)
	if h.DB == nil {
		t.Fatal("expected the handler's DB to be set")
	}
}

// TestScoutMissionSchema_CountBasedCapRecognizesMultipleActiveMissions
// verifies the COUNT-based query the dispatch gate now uses correctly
// tallies multiple simultaneously active scout_missions rows for the same
// encampment, and that the cap comparison behaves as doDispatchScoutMission
// expects at each step up to and including the cap.
func TestScoutMissionSchema_CountBasedCapRecognizesMultipleActiveMissions(t *testing.T) {
	db := rankingTestDB(t)

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(9401)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES (400, 400, 'plains', 'TestRegion', 'flat')
		RETURNING id`).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id) VALUES ($1, 'Scout Dispatch Camp', $2) RETURNING id`,
		int64(9401), coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}

	countActive := func() int {
		var n int
		_ = db.QueryRow("SELECT COUNT(*) FROM scout_missions WHERE encampment_id = $1 AND phase IN ('searching', 'returning')", campID).Scan(&n)
		return n
	}

	if got := countActive(); got != 0 {
		t.Fatalf("expected 0 active missions before dispatch, got %d", got)
	}

	for i := 1; i <= maxConcurrentScoutMissions; i++ {
		if _, err := db.Exec("INSERT INTO scout_missions (encampment_id, scouts_committed) VALUES ($1, 3)", campID); err != nil {
			t.Fatalf("seeding mission %d: %v", i, err)
		}
		if got := countActive(); got != i {
			t.Errorf("after seeding mission %d, expected count=%d, got %d", i, i, got)
		}
	}

	if got := countActive(); got < maxConcurrentScoutMissions {
		t.Errorf("expected count to reach the cap of %d, got %d", maxConcurrentScoutMissions, got)
	}
}
