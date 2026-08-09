package handlers

import (
	"context"
	"database/sql"
	"testing"
)

// seedMerchantOffer inserts a live (unpurchased, unexpired) merchant
// offer, returning its id.
func seedMerchantOffer(t *testing.T, db *sql.DB, itemType string, qty int, price float64) string {
	t.Helper()
	var offerID string
	if err := db.QueryRow(`
		INSERT INTO merchant_offers (item_type, quantity, price_dollars, expires_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP + INTERVAL '90 minutes') RETURNING id`,
		itemType, qty, price).Scan(&offerID); err != nil {
		t.Fatalf("seeding merchant offer: %v", err)
	}
	return offerID
}

// TestDoBuyMerchantOffer_PaysDollarsAndCreditsResource is the direct
// test for RARE_WORLD_FEATURES_PLAN.md Phase 4.4: buying an offer
// deducts exactly its price in dollars and credits exactly its
// quantity of the resource (storage-cap clamped), with no seller
// counterpart at all.
func TestDoBuyMerchantOffer_PaysDollarsAndCreditsResource(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &MerchantHandler{DB: db}

	campID := seedExchangeCamp(t, db, 43001, "Bargain Hunter Camp", 0, 0, 0)
	if _, err := db.Exec("UPDATE resources SET dollars = 1000 WHERE encampment_id = $1", campID); err != nil {
		t.Fatalf("seeding buyer dollars: %v", err)
	}
	offerID := seedMerchantOffer(t, db, "metal", 100, 180)

	msg, err := h.doBuyMerchantOffer(ctx, campID, offerID)
	if err != nil {
		t.Fatalf("doBuyMerchantOffer: unexpected error: %v (msg=%s)", err, msg)
	}

	var metal, dollars float64
	if err := db.QueryRow("SELECT metal, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&metal, &dollars); err != nil {
		t.Fatalf("reading buyer resources: %v", err)
	}
	if metal != 100 {
		t.Errorf("expected 100 metal credited, got %v", metal)
	}
	if dollars != 820 { // 1000 - 180
		t.Errorf("expected 820 dollars remaining, got %v", dollars)
	}

	var purchasedBy sql.NullString
	if err := db.QueryRow("SELECT purchased_by FROM merchant_offers WHERE id = $1", offerID).Scan(&purchasedBy); err != nil {
		t.Fatalf("reading offer: %v", err)
	}
	if !purchasedBy.Valid || purchasedBy.String != campID {
		t.Errorf("expected purchased_by to be %s, got %v", campID, purchasedBy)
	}
}

// TestDoBuyMerchantOffer_InsufficientFundsFails covers the balance
// check: a buyer without enough cash must not be charged or credited
// anything.
func TestDoBuyMerchantOffer_InsufficientFundsFails(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &MerchantHandler{DB: db}

	campID := seedExchangeCamp(t, db, 43002, "Broke Camp", 0, 0, 0)
	offerID := seedMerchantOffer(t, db, "crystal", 60, 500)

	if _, err := h.doBuyMerchantOffer(ctx, campID, offerID); err == nil {
		t.Error("expected doBuyMerchantOffer to fail for a buyer with insufficient funds")
	}

	var crystal, dollars float64
	if err := db.QueryRow("SELECT crystal, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&crystal, &dollars); err != nil {
		t.Fatalf("reading buyer resources: %v", err)
	}
	if crystal != 0 || dollars != 0 {
		t.Errorf("expected no resources to change on a failed purchase, got crystal=%v dollars=%v", crystal, dollars)
	}
}

// TestDoBuyMerchantOffer_AlreadyPurchasedFails covers race-safety: a
// second purchase attempt against an already-bought offer must fail
// cleanly rather than double-selling it.
func TestDoBuyMerchantOffer_AlreadyPurchasedFails(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &MerchantHandler{DB: db}

	winnerID := seedExchangeCamp(t, db, 43003, "Fast Buyer", 0, 0, 0)
	loserID := seedExchangeCamp(t, db, 43004, "Slow Buyer", 0, 0, 0)
	if _, err := db.Exec("UPDATE resources SET dollars = 1000 WHERE encampment_id IN ($1, $2)", winnerID, loserID); err != nil {
		t.Fatalf("seeding dollars: %v", err)
	}
	offerID := seedMerchantOffer(t, db, "scrap", 80, 100)

	if _, err := h.doBuyMerchantOffer(ctx, winnerID, offerID); err != nil {
		t.Fatalf("first purchase: unexpected error: %v", err)
	}
	if _, err := h.doBuyMerchantOffer(ctx, loserID, offerID); err == nil {
		t.Error("expected the second purchase attempt to fail")
	}

	var loserScrap, loserDollars float64
	if err := db.QueryRow("SELECT scrap, dollars FROM resources WHERE encampment_id = $1", loserID).Scan(&loserScrap, &loserDollars); err != nil {
		t.Fatalf("reading loser resources: %v", err)
	}
	if loserScrap != 0 || loserDollars != 1000 {
		t.Errorf("expected the loser to be untouched, got scrap=%v dollars=%v", loserScrap, loserDollars)
	}
}

// TestDoBuyMerchantOffer_ExpiredOfferFails confirms an offer past its
// expiry can no longer be purchased, even if it hasn't been swept by
// the expiry tick phase yet.
func TestDoBuyMerchantOffer_ExpiredOfferFails(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &MerchantHandler{DB: db}

	campID := seedExchangeCamp(t, db, 43005, "Late Camp", 0, 0, 0)
	if _, err := db.Exec("UPDATE resources SET dollars = 1000 WHERE encampment_id = $1", campID); err != nil {
		t.Fatalf("seeding dollars: %v", err)
	}
	var offerID string
	if err := db.QueryRow(`
		INSERT INTO merchant_offers (item_type, quantity, price_dollars, expires_at)
		VALUES ('metal', 50, 90, CURRENT_TIMESTAMP - INTERVAL '1 minute') RETURNING id`).Scan(&offerID); err != nil {
		t.Fatalf("seeding expired offer: %v", err)
	}

	if _, err := h.doBuyMerchantOffer(ctx, campID, offerID); err == nil {
		t.Error("expected doBuyMerchantOffer to fail for an expired offer")
	}
}
