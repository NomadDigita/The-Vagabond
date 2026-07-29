package handlers

import (
	"context"
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

// TestDoDispatchScoutMission_RejectsWhenAlreadyActive verifies the
// one-mission-at-a-time guard.
func TestDoDispatchScoutMission_RejectsWhenAlreadyActive(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedScoutDispatchCamp(t, 9603, 602, 602, 20)

	h := &ScoutMissionsHandler{DB: db}
	if _, err := h.doDispatchScoutMission(ctx, camp, 5); err != nil {
		t.Fatalf("first dispatch should succeed: %v", err)
	}
	if _, err := h.doDispatchScoutMission(ctx, camp, 5); err == nil {
		t.Error("expected the second dispatch to be rejected while a mission is already active")
	}
}

// TestRenderScoutStatus_ReflectsEachPhase verifies the beautified status
// text correctly reports each phase, and the "active" flag the panel
// uses to decide whether to show dispatch buttons.
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
	if active {
		t.Error("expected active=false with no mission dispatched")
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
	if !active {
		t.Error("expected active=true while searching")
	}
	if !strings.Contains(text, "still searching") {
		t.Errorf("expected searching-phase text, got %q", text)
	}

	// Returning phase.
	if _, err := db.Exec("UPDATE scout_missions SET phase = 'returning' WHERE encampment_id = $1", camp); err != nil {
		t.Fatalf("forcing returning phase: %v", err)
	}
	text, active, err = h.renderScoutStatus(ctx, camp)
	if err != nil {
		t.Fatalf("renderScoutStatus: %v", err)
	}
	if !active {
		t.Error("expected active=true while returning")
	}
	if !strings.Contains(text, "returning home") {
		t.Errorf("expected returning-phase text, got %q", text)
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

// TestScoutMissionSchema_OneMissionAtATimePerOutpost verifies the
// "already have a scout party out" guard's underlying query directly.
func TestScoutMissionSchema_OneMissionAtATimePerOutpost(t *testing.T) {
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

	var existingBefore bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM scout_missions WHERE encampment_id = $1 AND phase IN ('searching', 'returning'))", campID).Scan(&existingBefore)
	if existingBefore {
		t.Fatal("expected no active mission before dispatch")
	}

	if _, err := db.Exec("INSERT INTO scout_missions (encampment_id, scouts_committed) VALUES ($1, 3)", campID); err != nil {
		t.Fatalf("seeding mission: %v", err)
	}

	var existingAfter bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM scout_missions WHERE encampment_id = $1 AND phase IN ('searching', 'returning'))", campID).Scan(&existingAfter)
	if !existingAfter {
		t.Error("expected the newly-dispatched mission to be detected as active")
	}
}
