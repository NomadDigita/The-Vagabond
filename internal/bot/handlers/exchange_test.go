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
		msg, err := h.doPostListing(ctx, campID, tc.resource, tc.qty, tc.price)
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

	if _, err := h.doPostListing(ctx, campID, "metal", 200, 400); err != nil {
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

	if _, err := h.doPostListing(ctx, campID, "metal", 200, 400); err == nil {
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

	if _, err := h.doPostListing(ctx, campID, "rations", 10, 5); err == nil {
		t.Fatal("expected an error for a resource that isn't on the exchange allow-list")
	}
}

func TestDoPostListing_NonPositiveQuantityOrPriceRejected(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ExchangeHandler{DB: db}

	campID := seedExchangeCamp(t, db, 40005, "Edge Camp", 1000, 1000, 1000)

	if _, err := h.doPostListing(ctx, campID, "metal", 0, 100); err == nil {
		t.Error("expected an error for zero quantity")
	}
	if _, err := h.doPostListing(ctx, campID, "metal", 10, 0); err == nil {
		t.Error("expected an error for zero price")
	}
	if _, err := h.doPostListing(ctx, campID, "metal", -5, 100); err == nil {
		t.Error("expected an error for negative quantity")
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
