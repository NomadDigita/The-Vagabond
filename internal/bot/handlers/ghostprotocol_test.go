package handlers

import (
	"context"
	"testing"
	"time"
)


// TestDoGhostProtocol_DeductsProportionalCostAndClearsLocks verifies the
// core mechanic from AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section
// 3.4: relocates the camp, deducts ghostProtocolCostFraction of current
// resources (not a flat number), and deletes every known_locations row
// targeting this camp while leaving encampment_discoveries untouched.
func TestDoGhostProtocol_DeductsProportionalCostAndClearsLocksButNotDiscoveries(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(9301)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES (300, 300, 'plains', 'TestRegion', 'flat')
		RETURNING id`).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id) VALUES ($1, 'Ghost Test Camp', $2) RETURNING id`,
		int64(9301), coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO resources (encampment_id, scrap, metal, crystal, dollars) VALUES ($1, 1000, 2000, 300, 4000)", campID); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}

	var observerCampID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES (301, 301, 'plains', 'TestRegion', 'flat')
		RETURNING id`).Scan(&observerCampID); err != nil {
		t.Fatalf("seeding observer coordinate: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(9302)); err != nil {
		t.Fatalf("seeding observer user: %v", err)
	}
	var observerID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id) VALUES ($1, 'Observer Camp', $2) RETURNING id`,
		int64(9302), observerCampID).Scan(&observerID); err != nil {
		t.Fatalf("seeding observer encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO known_locations (observer_encampment_id, target_encampment_id, x, y, region) VALUES ($1, $2, 300, 300, 'TestRegion')", observerID, campID); err != nil {
		t.Fatalf("seeding known_locations: %v", err)
	}
	if _, err := db.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'manual')", observerID, campID); err != nil {
		t.Fatalf("seeding encampment_discoveries: %v", err)
	}

	h := &JobsHandler{DB: db}
	msg, err := h.doGhostProtocol(ctx, campID)
	if err != nil {
		t.Fatalf("doGhostProtocol: %v (%s)", err, msg)
	}

	var scrap, metal, crystal, dollars float64
	if err := db.QueryRow("SELECT scrap, metal, crystal, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&scrap, &metal, &crystal, &dollars); err != nil {
		t.Fatalf("reading post-flee resources: %v", err)
	}
	if scrap != 500 || metal != 1000 || crystal != 150 || dollars != 2000 {
		t.Errorf("expected exactly 50%% of each resource deducted, got scrap=%v metal=%v crystal=%v dollars=%v", scrap, metal, crystal, dollars)
	}

	var newCoordID string
	if err := db.QueryRow("SELECT coordinate_id FROM encampments WHERE id = $1", campID).Scan(&newCoordID); err != nil {
		t.Fatalf("reading post-flee coordinate: %v", err)
	}
	if newCoordID == coordID {
		t.Error("expected the camp to have relocated to a new coordinate")
	}

	var knownLocationsCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM known_locations WHERE target_encampment_id = $1", campID).Scan(&knownLocationsCount)
	if knownLocationsCount != 0 {
		t.Error("expected all known_locations rows targeting this camp to be deleted")
	}

	var discoveryCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampment_discoveries WHERE target_encampment_id = $1", campID).Scan(&discoveryCount)
	if discoveryCount == 0 {
		t.Error("expected encampment_discoveries to be left untouched (it's a different, weaker kind of knowledge)")
	}

	// Second call should now be on cooldown.
	if _, err := h.doGhostProtocol(ctx, campID); err == nil {
		t.Error("expected a second immediate Ghost Protocol call to be rejected by the cooldown")
	}
}

// TestDoGhostProtocol_RespectsExistingCooldown covers the cooldown gate
// directly, independent of having just executed once.
func TestDoGhostProtocol_RespectsExistingCooldown(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(9303)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES (302, 302, 'plains', 'TestRegion', 'flat')
		RETURNING id`).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id, last_ghost_protocol_at)
		VALUES ($1, 'Recent Ghost Camp', $2, $3) RETURNING id`,
		int64(9303), coordID, time.Now().UTC().Add(-24*time.Hour)).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}

	h := &JobsHandler{DB: db}
	if _, err := h.doGhostProtocol(ctx, campID); err == nil {
		t.Error("expected Ghost Protocol to be rejected: last use was only 1 day ago, cooldown is 90 days")
	}
}
