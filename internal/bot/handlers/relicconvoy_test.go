package handlers

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
)

// raiseStorageCap gives a test camp enough storage headroom
// (extension_lvl 40 -> a 40,500 cap, see storagecap.Cap's formula) for
// tests that credit reward amounts larger than a fresh level-1 camp's
// default 500 cap would allow through un-clamped.
func raiseStorageCap(t *testing.T, db *sql.DB, campID string) {
	t.Helper()
	if _, err := db.Exec("UPDATE encampments SET extension_lvl = 40 WHERE id = $1", campID); err != nil {
		t.Fatalf("raising storage cap: %v", err)
	}
}

// relicCoordCounter guarantees a unique (x, y) per seedRelicConvoy call
// within a test run - coordinates.(x, y) is UNIQUE, and deriving x from
// len(name) (an earlier draft of this helper) risked collisions between
// same-length relic names.
var relicCoordCounter int64 = 80000

// seedRelicConvoy inserts a live (unclaimed, unexpired) relic convoy
// anchored to a fresh test coordinate, returning its id.
func seedRelicConvoy(t *testing.T, db *sql.DB, name string, reward float64) string {
	t.Helper()
	x := atomic.AddInt64(&relicCoordCounter, 1)
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain)
		VALUES ($1, $1, 'plains', 'TestRegion', 'flat') RETURNING id`, x).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var convoyID string
	if err := db.QueryRow(`
		INSERT INTO relic_convoys (relic_name, coordinate_id, reward_dollars, expires_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP + INTERVAL '3 hours') RETURNING id`,
		name, coordID, reward).Scan(&convoyID); err != nil {
		t.Fatalf("seeding relic convoy: %v", err)
	}
	return convoyID
}

// TestDoClaimRelicConvoy_FirstClaimSetsTitleAndPaysReward is the direct
// test for RARE_WORLD_FEATURES_PLAN.md Phase 1.3: claiming a relic pays
// the reward (storage-cap clamped like every other resource-gain path)
// and, since this is the encampment's first relic, sets a permanent
// relic_title.
func TestDoClaimRelicConvoy_FirstClaimSetsTitleAndPaysReward(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &RelicConvoyHandler{DB: db}

	campID := seedExchangeCamp(t, db, 42001, "Relic Hunter Camp", 0, 0, 0)
	raiseStorageCap(t, db, campID)
	convoyID := seedRelicConvoy(t, db, "The Chrome Reliquary", 15000)

	msg, err := h.doClaimRelicConvoy(ctx, campID, convoyID)
	if err != nil {
		t.Fatalf("doClaimRelicConvoy: unexpected error: %v (msg=%s)", err, msg)
	}

	var dollars float64
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", campID).Scan(&dollars); err != nil {
		t.Fatalf("reading buyer resources: %v", err)
	}
	if dollars != 15000 {
		t.Errorf("expected 15000 dollars credited, got %v", dollars)
	}

	var claimedBy sql.NullString
	if err := db.QueryRow("SELECT claimed_by FROM relic_convoys WHERE id = $1", convoyID).Scan(&claimedBy); err != nil {
		t.Fatalf("reading convoy: %v", err)
	}
	if !claimedBy.Valid || claimedBy.String != campID {
		t.Errorf("expected convoy claimed_by to be %s, got %v", campID, claimedBy)
	}

	var title sql.NullString
	if err := db.QueryRow("SELECT relic_title FROM encampments WHERE id = $1", campID).Scan(&title); err != nil {
		t.Fatalf("reading relic title: %v", err)
	}
	if !title.Valid || title.String == "" {
		t.Error("expected a relic_title to be set after the first claim")
	}
}

// TestDoClaimRelicConvoy_SecondClaimNeverOverwritesTitle covers the
// plan doc's explicit "first relic keeps the title" rule: an
// encampment that already has a relic_title from an earlier claim must
// keep that exact title after claiming a second, different relic.
func TestDoClaimRelicConvoy_SecondClaimNeverOverwritesTitle(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &RelicConvoyHandler{DB: db}

	campID := seedExchangeCamp(t, db, 42002, "Serial Relic Hunter", 0, 0, 0)
	raiseStorageCap(t, db, campID)
	firstConvoy := seedRelicConvoy(t, db, "The Last Aurora Convoy", 12000)
	if _, err := h.doClaimRelicConvoy(ctx, campID, firstConvoy); err != nil {
		t.Fatalf("first doClaimRelicConvoy: unexpected error: %v", err)
	}

	var firstTitle string
	if err := db.QueryRow("SELECT relic_title FROM encampments WHERE id = $1", campID).Scan(&firstTitle); err != nil {
		t.Fatalf("reading first title: %v", err)
	}

	secondConvoy := seedRelicConvoy(t, db, "The Obsidian Vanguard", 18000)
	if _, err := h.doClaimRelicConvoy(ctx, campID, secondConvoy); err != nil {
		t.Fatalf("second doClaimRelicConvoy: unexpected error: %v", err)
	}

	var secondTitle string
	if err := db.QueryRow("SELECT relic_title FROM encampments WHERE id = $1", campID).Scan(&secondTitle); err != nil {
		t.Fatalf("reading second title: %v", err)
	}
	if secondTitle != firstTitle {
		t.Errorf("expected relic_title to stay %q after a second claim, got %q", firstTitle, secondTitle)
	}

	// Both claims should still have paid out though - the title rule
	// only protects the title field, not the reward.
	var dollars float64
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", campID).Scan(&dollars); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if dollars != 30000 {
		t.Errorf("expected both rewards (12000+18000=30000) credited, got %v", dollars)
	}
}

// TestDoClaimRelicConvoy_AlreadyClaimedFails covers the race-safety
// requirement: a second claim attempt against an already-claimed
// convoy must fail cleanly rather than double-pay.
func TestDoClaimRelicConvoy_AlreadyClaimedFails(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &RelicConvoyHandler{DB: db}

	winnerID := seedExchangeCamp(t, db, 42003, "Fast Camp", 0, 0, 0)
	loserID := seedExchangeCamp(t, db, 42004, "Slow Camp", 0, 0, 0)
	convoyID := seedRelicConvoy(t, db, "The Hollow Crown Convoy", 16000)

	if _, err := h.doClaimRelicConvoy(ctx, winnerID, convoyID); err != nil {
		t.Fatalf("first claim: unexpected error: %v", err)
	}

	if _, err := h.doClaimRelicConvoy(ctx, loserID, convoyID); err == nil {
		t.Error("expected the second claim attempt to fail, but it succeeded")
	}

	var loserDollars float64
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", loserID).Scan(&loserDollars); err != nil {
		t.Fatalf("reading loser resources: %v", err)
	}
	if loserDollars != 0 {
		t.Errorf("expected the loser to receive nothing, got %v", loserDollars)
	}
}
