package handlers

import (
	"testing"
)

// TestScoutMissionsHandler_Construction is a minimal smoke test - the
// handler's real logic needs a telebot.Context to exercise directly, so
// the core mechanics are covered end-to-end in
// internal/engine/tick/scoutmissions_test.go instead. This just confirms
// the handler wires up without panicking given a real DB, matching the
// smoke-test convention used elsewhere for telebot-dependent handlers.
func TestScoutMissionsHandler_Construction(t *testing.T) {
	db := rankingTestDB(t)
	h := NewScoutMissionsHandler(db)
	if h.DB == nil {
		t.Fatal("expected the handler's DB to be set")
	}
}

// TestScoutMissionSchema_OneMissionAtATimePerOutpost verifies the
// "already have a scout party out" guard's underlying query directly:
// an outpost with an active (searching or returning) mission should be
// detected as such.
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
