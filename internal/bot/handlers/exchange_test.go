package handlers

import (
	"context"
	"database/sql"
	"testing"
)

func seedExchangeCamp(t *testing.T, db *sql.DB, telegramID int64, name string, metal, crystal, scrap float64) string {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $1, 'plains', 'TestRegion', 'flat')
		RETURNING id`, telegramID).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, level, coordinate_id)
		VALUES ($1, $2, 1, $3) RETURNING id`, telegramID, name, coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO resources (encampment_id, metal, crystal, scrap) VALUES ($1, $2, $3, $4)`,
		campID, metal, crystal, scrap); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}
	return campID
}

// TestDoPostListing_MetalCrystalScrapAllTradeable is the direct test
// for Milestone 3's extension of the market: unlike the old
// HandlePostListingCallback (which only ever offered two fixed
// buttons), doPostListing must accept an arbitrary quantity/price for
// any of the three real tradeable resources - including Scrap, since
// the plan doc's own literal example is "list 300k scrap for sale".
func TestDoPostListing_MetalCrystalScrapAllTradeable(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40001, "Trader Camp", 1000, 1000, 500000)

	cases := []struct {
		resource string
		qty      int
		price    float64
	}{
		{"metal", 200, 400},
		{"crystal", 50, 900},
		{"scrap", 300000, 500},
	}
	for _, tc := range cases {
		msg, err := h.doPostListing(ctx, campID, tc.resource, tc.qty, "dollars", tc.price)
		if err != nil {
			t.Fatalf("doPostListing(%s): unexpected error: %v (msg=%s)", tc.resource, err, msg)
		}

		var count int
		if err := db.QueryRow(`
			SELECT COUNT(*) FROM market_exchange
			WHERE seller_id = $1 AND item_type = $2 AND quantity = $3 AND price_dollars = $4`,
			campID, tc.resource, tc.qty, tc.price).Scan(&count); err != nil {
			t.Fatalf("querying market_exchange: %v", err)
		}
		if count != 1 {
			t.Errorf("expected exactly one %s listing at qty=%d price=%v, got %d", tc.resource, tc.qty, tc.price, count)
		}
	}
}

func TestDoPostListing_DeductsFromReserves(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40002, "Deduct Camp", 500, 0, 0)

	if _, err := h.doPostListing(ctx, campID, "metal", 200, "dollars", 400); err != nil {
		t.Fatalf("doPostListing: unexpected error: %v", err)
	}

	var remaining float64
	if err := db.QueryRow("SELECT metal FROM resources WHERE encampment_id = $1", campID).Scan(&remaining); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if remaining != 300 {
		t.Errorf("expected 500-200=300 metal remaining, got %v", remaining)
	}
}

func TestDoPostListing_InsufficientResourceRejected(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40003, "Poor Camp", 10, 0, 0)

	if _, err := h.doPostListing(ctx, campID, "metal", 200, "dollars", 400); err == nil {
		t.Fatal("expected an error when listing more metal than the camp has")
	}

	var remaining float64
	if err := db.QueryRow("SELECT metal FROM resources WHERE encampment_id = $1", campID).Scan(&remaining); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if remaining != 10 {
		t.Errorf("expected reserves untouched at 10, got %v", remaining)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM market_exchange WHERE seller_id = $1", campID).Scan(&count)
	if count != 0 {
		t.Errorf("expected no listing to be created on a rejected post, got %d", count)
	}
}

func TestDoPostListing_UnsupportedResourceRejected(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40004, "Rations Camp", 0, 0, 0)

	if _, err := h.doPostListing(ctx, campID, "rations", 10, "dollars", 5); err == nil {
		t.Fatal("expected an error for a resource that isn't on the exchange allow-list")
	}
}

func TestDoPostListing_NonPositiveQuantityOrPriceRejected(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40005, "Edge Camp", 1000, 1000, 1000)

	if _, err := h.doPostListing(ctx, campID, "metal", 0, "dollars", 100); err == nil {
		t.Error("expected an error for zero quantity")
	}
	if _, err := h.doPostListing(ctx, campID, "metal", 10, "dollars", 0); err == nil {
		t.Error("expected an error for zero price")
	}
	if _, err := h.doPostListing(ctx, campID, "metal", -5, "dollars", 100); err == nil {
		t.Error("expected an error for negative quantity")
	}
}

// TestDoPostListing_BarterAskType covers the market's extension beyond
// cash-only listings: a seller can ask for another resource instead
// of dollars, and the row's ask_type/ask_quantity (not price_dollars,
// which stays 0 for barter rows) is what a buyer must actually pay.
func TestDoPostListing_BarterAskType(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40006, "Barter Camp", 0, 0, 500000)

	msg, err := h.doPostListing(ctx, campID, "scrap", 200000, "metal", 40000)
	if err != nil {
		t.Fatalf("doPostListing (barter): unexpected error: %v (msg=%s)", err, msg)
	}

	var askType string
	var askQty, priceDollars float64
	if err := db.QueryRow(`
		SELECT ask_type, ask_quantity, price_dollars FROM market_exchange
		WHERE seller_id = $1 AND item_type = 'scrap'`, campID).Scan(&askType, &askQty, &priceDollars); err != nil {
		t.Fatalf("reading listing row: %v", err)
	}
	if askType != "metal" {
		t.Errorf("expected ask_type 'metal', got %q", askType)
	}
	if askQty != 40000 {
		t.Errorf("expected ask_quantity 40000, got %v", askQty)
	}
	if priceDollars != 0 {
		t.Errorf("expected price_dollars=0 for a barter row, got %v", priceDollars)
	}
}

func TestDoPostListing_BarterCannotAskForSameResource(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40007, "Self Barter Camp", 1000, 0, 0)

	if _, err := h.doPostListing(ctx, campID, "metal", 100, "metal", 50); err == nil {
		t.Fatal("expected an error when asking for the same resource being listed")
	}
}

func TestDoPostListing_BarterRejectsUnsupportedAskType(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40008, "Bad Ask Camp", 1000, 0, 0)

	if _, err := h.doPostListing(ctx, campID, "metal", 100, "rations", 50); err == nil {
		t.Fatal("expected an error for an ask_type not on the tradeable allow-list")
	}
}

func TestFormatAsk(t *testing.T) {
	if got := formatAsk("dollars", 500); got != "$500" {
		t.Errorf("expected \"$500\", got %q", got)
	}
	if got := formatAsk("metal", 40000); got != "40000 metal" {
		t.Errorf("expected \"40000 metal\", got %q", got)
	}
}

// TestDoBuyListingQty_PartialFillLeavesRemainder is the direct test for
// the "buy 500 out of an 800 listing" feature request: buying less
// than a listing's full quantity should only take exactly what was
// asked for, pro-rate the price, and leave the rest of the listing
// live (not sold) for the next buyer.
func TestDoBuyListingQty_PartialFillLeavesRemainder(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	sellerID := seedExchangeCamp(t, db, 41001, "Seller Camp", 800, 0, 0)
	buyerID := seedExchangeCamp(t, db, 41002, "Buyer Camp", 0, 0, 0)
	if _, err := db.Exec("UPDATE resources SET dollars = 10000 WHERE encampment_id = $1", buyerID); err != nil {
		t.Fatalf("seeding buyer dollars: %v", err)
	}

	// 800 metal listed for $1600 total -> $2/unit.
	if _, err := h.doPostListing(ctx, sellerID, "metal", 800, "dollars", 1600); err != nil {
		t.Fatalf("doPostListing: unexpected error: %v", err)
	}
	var listingID string
	if err := db.QueryRow("SELECT id FROM market_exchange WHERE seller_id = $1", sellerID).Scan(&listingID); err != nil {
		t.Fatalf("reading listing id: %v", err)
	}

	if _, err := h.doBuyListingQty(ctx, buyerID, listingID, 500); err != nil {
		t.Fatalf("doBuyListingQty(500 of 800): unexpected error: %v", err)
	}

	var buyerMetal, buyerDollars float64
	if err := db.QueryRow("SELECT metal, dollars FROM resources WHERE encampment_id = $1", buyerID).Scan(&buyerMetal, &buyerDollars); err != nil {
		t.Fatalf("reading buyer resources: %v", err)
	}
	if buyerMetal != 500 {
		t.Errorf("expected buyer to receive exactly 500 metal, got %v", buyerMetal)
	}
	if buyerDollars != 9000 { // 10000 - (500 * $2/unit)
		t.Errorf("expected buyer to pay $1000 for 500 units at $2/unit (9000 remaining), got %v", buyerDollars)
	}

	var remQty int
	var remAsk float64
	var isSold bool
	if err := db.QueryRow("SELECT quantity, ask_quantity, is_sold FROM market_exchange WHERE id = $1", listingID).Scan(&remQty, &remAsk, &isSold); err != nil {
		t.Fatalf("reading remaining listing: %v", err)
	}
	if isSold {
		t.Error("expected the listing to remain active (not sold) after a partial fill")
	}
	if remQty != 300 {
		t.Errorf("expected 800-500=300 metal remaining on the listing, got %v", remQty)
	}
	if remAsk != 600 { // 1600 - 1000
		t.Errorf("expected $600 asking remaining on the listing, got %v", remAsk)
	}
}

// TestDoBuyListingQty_FullFillStillMarksSold covers the existing
// whole-listing behavior: requesting at least the full quantity (or 0,
// via doBuyListing) still buys everything and marks the listing sold,
// exactly as before partial fills were introduced.
func TestDoBuyListingQty_FullFillStillMarksSold(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	sellerID := seedExchangeCamp(t, db, 41003, "Seller Camp 2", 200, 0, 0)
	buyerID := seedExchangeCamp(t, db, 41004, "Buyer Camp 2", 0, 0, 0)
	if _, err := db.Exec("UPDATE resources SET dollars = 1000 WHERE encampment_id = $1", buyerID); err != nil {
		t.Fatalf("seeding buyer dollars: %v", err)
	}

	if _, err := h.doPostListing(ctx, sellerID, "metal", 200, "dollars", 400); err != nil {
		t.Fatalf("doPostListing: unexpected error: %v", err)
	}
	var listingID string
	if err := db.QueryRow("SELECT id FROM market_exchange WHERE seller_id = $1", sellerID).Scan(&listingID); err != nil {
		t.Fatalf("reading listing id: %v", err)
	}

	// Requesting more than what's listed (300 > 200) clamps to a full buy.
	if _, err := h.doBuyListingQty(ctx, buyerID, listingID, 300); err != nil {
		t.Fatalf("doBuyListingQty: unexpected error: %v", err)
	}

	var buyerMetal, buyerDollars float64
	if err := db.QueryRow("SELECT metal, dollars FROM resources WHERE encampment_id = $1", buyerID).Scan(&buyerMetal, &buyerDollars); err != nil {
		t.Fatalf("reading buyer resources: %v", err)
	}
	if buyerMetal != 200 {
		t.Errorf("expected buyer to receive all 200 metal, got %v", buyerMetal)
	}
	if buyerDollars != 600 {
		t.Errorf("expected buyer to pay the full $400, got %v remaining", buyerDollars)
	}

	var isSold bool
	if err := db.QueryRow("SELECT is_sold FROM market_exchange WHERE id = $1", listingID).Scan(&isSold); err != nil {
		t.Fatalf("reading listing sold flag: %v", err)
	}
	if !isSold {
		t.Error("expected a full-quantity buy to mark the listing sold")
	}
}

// TestDoBuyMarketItem_PartialFillFromLargerListing is doBuyMarketItem's
// (the natural-language "buy X" search path) counterpart to
// TestDoBuyListingQty_PartialFillLeavesRemainder: wanting 500 metal
// should succeed and only take 500 even when the only matching listing
// has 800, and the per-unit price filter should admit a listing whose
// FULL price exceeds the budget but whose price for just the requested
// quantity does not.
func TestDoBuyMarketItem_PartialFillFromLargerListing(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	sellerID := seedExchangeCamp(t, db, 41005, "Bulk Seller", 800, 0, 0)
	buyerID := seedExchangeCamp(t, db, 41006, "Bulk Buyer", 0, 0, 0)
	if _, err := db.Exec("UPDATE resources SET dollars = 2000 WHERE encampment_id = $1", buyerID); err != nil {
		t.Fatalf("seeding buyer dollars: %v", err)
	}

	// 800 metal for $2400 total ($3/unit) - the whole lot costs more
	// than the buyer's $2000 budget, but 500 units at $3/unit ($1500)
	// fits comfortably.
	if _, err := h.doPostListing(ctx, sellerID, "metal", 800, "dollars", 2400); err != nil {
		t.Fatalf("doPostListing: unexpected error: %v", err)
	}

	msg, err := h.doBuyMarketItem(ctx, buyerID, "metal", 500, 2000)
	if err != nil {
		t.Fatalf("doBuyMarketItem(500 metal, $2000 budget): unexpected error: %v (msg=%s)", err, msg)
	}

	var buyerMetal, buyerDollars float64
	if err := db.QueryRow("SELECT metal, dollars FROM resources WHERE encampment_id = $1", buyerID).Scan(&buyerMetal, &buyerDollars); err != nil {
		t.Fatalf("reading buyer resources: %v", err)
	}
	if buyerMetal != 500 {
		t.Errorf("expected exactly 500 metal bought, got %v", buyerMetal)
	}
	if buyerDollars != 500 { // 2000 - 1500
		t.Errorf("expected $1500 spent (500 remaining), got %v remaining", buyerDollars)
	}

	var remQty int
	if err := db.QueryRow("SELECT quantity FROM market_exchange WHERE seller_id = $1", sellerID).Scan(&remQty); err != nil {
		t.Fatalf("reading remaining listing: %v", err)
	}
	if remQty != 300 {
		t.Errorf("expected 300 metal left on the listing, got %v", remQty)
	}
}

func TestMarketResourceColumn(t *testing.T) {
	for _, ok := range []string{"metal", "Metal", "CRYSTAL", " scrap "} {
		if _, valid := marketResourceColumn(ok); !valid {
			t.Errorf("expected %q to resolve to a valid market resource column", ok)
		}
	}
	for _, bad := range []string{"rations", "dollars", "", "gold"} {
		if _, valid := marketResourceColumn(bad); valid {
			t.Errorf("expected %q to be rejected as a market resource", bad)
		}
	}
}
