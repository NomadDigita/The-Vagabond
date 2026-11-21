package handlers

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
)

func raiseStorageCap(t *testing.T, db *sql.DB, campID string) {
	t.Helper()
	if _, err := db.Exec("UPDATE encampments SET extension_lvl = 40 WHERE id = $1", campID); err != nil {
		t.Fatalf("raising storage cap: %v", err)
	}
}

var relicCoordCounter int64 = 80000

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
	if dollars != 18000 {
		t.Errorf("expected 18000 dollars credited (15000 relic reward + 3000 relic_hunter achievement), got %v", dollars)
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

	var dollars float64
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", campID).Scan(&dollars); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if dollars != 33000 {
		t.Errorf("expected 33000 dollars (12000+18000 relic rewards + 3000 one-time relic_hunter achievement), got %v", dollars)
	}
}

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
