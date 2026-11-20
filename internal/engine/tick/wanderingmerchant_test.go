package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestWanderingMerchantSpawn_OnlyOneActiveBatchAtATime(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	oldChance := merchantSpawnChance
	merchantSpawnChance = 1.0
	t.Cleanup(func() { merchantSpawnChance = oldChance })

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.wanderingMerchantSpawn(ctx, tx) })

	var countAfterFirst int
	if err := db.QueryRow("SELECT COUNT(*) FROM merchant_offers WHERE purchased_by IS NULL AND expires_at > CURRENT_TIMESTAMP").Scan(&countAfterFirst); err != nil {
		t.Fatalf("counting merchant offers: %v", err)
	}
	if countAfterFirst < merchantMinOffers || countAfterFirst > merchantMaxOffers {
		t.Fatalf("expected between %d and %d offers after the first spawn, got %d", merchantMinOffers, merchantMaxOffers, countAfterFirst)
	}

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.wanderingMerchantSpawn(ctx, tx) })

	var countAfterSecond int
	if err := db.QueryRow("SELECT COUNT(*) FROM merchant_offers").Scan(&countAfterSecond); err != nil {
		t.Fatalf("counting merchant offers: %v", err)
	}
	if countAfterSecond != countAfterFirst {
		t.Errorf("expected the second spawn phase to be a no-op (still %d total offers), got %d", countAfterFirst, countAfterSecond)
	}
}

func TestWanderingMerchantSpawn_NeverRollsWhenChanceIsZero(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	oldChance := merchantSpawnChance
	merchantSpawnChance = 0.0
	t.Cleanup(func() { merchantSpawnChance = oldChance })

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.wanderingMerchantSpawn(ctx, tx) })

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM merchant_offers").Scan(&count); err != nil {
		t.Fatalf("counting merchant offers: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no merchant offers to spawn at 0%% chance, got %d", count)
	}
}

func TestWanderingMerchantExpire_ClearsOnlyUnpurchasedExpiredOffers(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	buyerID := seedEncampment(t, db, 53001, "Buyer Camp", 30, 30, "TestRegion", false)

	var expiredID, purchasedID, liveID string
	if err := db.QueryRow(`
		INSERT INTO merchant_offers (item_type, quantity, price_dollars, expires_at)
		VALUES ('metal', 50, 90, CURRENT_TIMESTAMP - INTERVAL '1 minute') RETURNING id`).Scan(&expiredID); err != nil {
		t.Fatalf("seeding expired offer: %v", err)
	}
	if err := db.QueryRow(`
		INSERT INTO merchant_offers (item_type, quantity, price_dollars, expires_at, purchased_by, purchased_at)
		VALUES ('crystal', 60, 500, CURRENT_TIMESTAMP - INTERVAL '1 minute', $1, CURRENT_TIMESTAMP) RETURNING id`, buyerID).Scan(&purchasedID); err != nil {
		t.Fatalf("seeding purchased offer: %v", err)
	}
	if err := db.QueryRow(`
		INSERT INTO merchant_offers (item_type, quantity, price_dollars, expires_at)
		VALUES ('scrap', 80, 100, CURRENT_TIMESTAMP + INTERVAL '1 hour') RETURNING id`).Scan(&liveID); err != nil {
		t.Fatalf("seeding live offer: %v", err)
	}

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.wanderingMerchantExpire(ctx, tx) })

	var stillExists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM merchant_offers WHERE id = $1)", expiredID).Scan(&stillExists)
	if stillExists {
		t.Error("expected the expired, unpurchased offer to be deleted")
	}
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM merchant_offers WHERE id = $1)", purchasedID).Scan(&stillExists)
	if !stillExists {
		t.Error("expected the purchased offer to survive expiry")
	}
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM merchant_offers WHERE id = $1)", liveID).Scan(&stillExists)
	if !stillExists {
		t.Error("expected the still-live offer to survive")
	}
}
