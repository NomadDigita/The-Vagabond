package gear

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/NomadDigita/The-Vagabond/internal/db/schema"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database gear test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range schema.Statements() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("applying schema: %v\n%s", err, stmt)
		}
	}
	if _, err := db.Exec(`TRUNCATE player_gear, crafting_queue, resources, encampments, coordinates, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

func seedGearPlayer(t *testing.T, db *sql.DB, userID int64) string {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", userID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $1, 'plains', 'TestRegion', 'flat')
		RETURNING id`, userID).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id) VALUES ($1, 'Gear Test Camp', $2)
		RETURNING id`, userID, coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO resources (encampment_id, scrap, metal, crystal) VALUES ($1, 5000, 5000, 5000)", campID); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}
	return campID
}

func TestStartCraft_DeductsCostAndQueues(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70001)

	if _, err := StartCraft(ctx, db, campID, "scrap_blade"); err != nil {
		t.Fatalf("StartCraft: unexpected error: %v", err)
	}

	var scrap float64
	if err := db.QueryRow("SELECT scrap FROM resources WHERE encampment_id = $1", campID).Scan(&scrap); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if scrap != 4500 {
		t.Errorf("expected 4500 scrap remaining, got %v", scrap)
	}

	var pending int
	if err := db.QueryRow("SELECT COUNT(*) FROM crafting_queue WHERE encampment_id = $1 AND is_completed = FALSE", campID).Scan(&pending); err != nil {
		t.Fatalf("counting crafting queue: %v", err)
	}
	if pending != 1 {
		t.Errorf("expected 1 pending crafting_queue row, got %d", pending)
	}

	var quantity int
	_ = db.QueryRow("SELECT quantity FROM player_gear WHERE encampment_id = $1 AND gear_key = 'scrap_blade'", campID).Scan(&quantity)
	if quantity != 0 {
		t.Errorf("expected the item to NOT be granted yet (still crafting), got quantity %d", quantity)
	}
}

func TestStartCraft_InsufficientResourcesFails(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70002)
	if _, err := db.Exec("UPDATE resources SET scrap = 100 WHERE encampment_id = $1", campID); err != nil {
		t.Fatalf("lowering scrap: %v", err)
	}

	if _, err := StartCraft(ctx, db, campID, "scrap_blade"); err == nil {
		t.Error("expected StartCraft to fail with insufficient scrap")
	}

	var scrap float64
	_ = db.QueryRow("SELECT scrap FROM resources WHERE encampment_id = $1", campID).Scan(&scrap)
	if scrap != 100 {
		t.Errorf("expected scrap to remain untouched at 100, got %v", scrap)
	}
}

func TestStartCraft_RejectsNonCraftableGear(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70003)

	if _, err := StartCraft(ctx, db, campID, "arc_cannon"); err == nil {
		t.Error("expected StartCraft to reject a non-craftable (Epic) item")
	}
}

func TestResolveCraftingQueue_GrantsItemOnceComplete(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70004)

	if _, err := db.Exec(`
		INSERT INTO crafting_queue (encampment_id, gear_key, completes_at)
		VALUES ($1, 'scrap_blade', CURRENT_TIMESTAMP - INTERVAL '1 minute')`, campID); err != nil {
		t.Fatalf("seeding completed crafting_queue row: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := ResolveCraftingQueue(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("ResolveCraftingQueue: unexpected error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var quantity int
	if err := db.QueryRow("SELECT quantity FROM player_gear WHERE encampment_id = $1 AND gear_key = 'scrap_blade'", campID).Scan(&quantity); err != nil {
		t.Fatalf("reading player_gear: %v", err)
	}
	if quantity != 1 {
		t.Errorf("expected quantity 1 after resolution, got %d", quantity)
	}

	var isCompleted bool
	if err := db.QueryRow("SELECT is_completed FROM crafting_queue WHERE encampment_id = $1 AND gear_key = 'scrap_blade'", campID).Scan(&isCompleted); err != nil {
		t.Fatalf("reading crafting_queue: %v", err)
	}
	if !isCompleted {
		t.Error("expected the crafting_queue row to be marked completed")
	}
}

func TestResolveCraftingQueue_LeavesStillPendingCraftsAlone(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70005)

	if _, err := db.Exec(`
		INSERT INTO crafting_queue (encampment_id, gear_key, completes_at)
		VALUES ($1, 'scrap_blade', CURRENT_TIMESTAMP + INTERVAL '1 hour')`, campID); err != nil {
		t.Fatalf("seeding not-yet-complete crafting_queue row: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := ResolveCraftingQueue(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("ResolveCraftingQueue: unexpected error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var quantity int
	_ = db.QueryRow("SELECT quantity FROM player_gear WHERE encampment_id = $1 AND gear_key = 'scrap_blade'", campID).Scan(&quantity)
	if quantity != 0 {
		t.Errorf("expected the not-yet-complete craft to grant nothing, got quantity %d", quantity)
	}
}

func TestEquip_SwapsWithinSlotAndRequiresOwnership(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70006)

	if err := Equip(ctx, db, campID, "scrap_blade"); err == nil {
		t.Error("expected equipping an unowned item to fail")
	}

	for _, key := range []string{"scrap_blade", "salvaged_rifle"} {
		if _, err := db.Exec(`
			INSERT INTO player_gear (encampment_id, gear_key, quantity) VALUES ($1, $2, 1)`, campID, key); err != nil {
			t.Fatalf("seeding owned gear %s: %v", key, err)
		}
	}

	if err := Equip(ctx, db, campID, "scrap_blade"); err != nil {
		t.Fatalf("Equip scrap_blade: unexpected error: %v", err)
	}
	if err := Equip(ctx, db, campID, "salvaged_rifle"); err != nil {
		t.Fatalf("Equip salvaged_rifle: unexpected error: %v", err)
	}

	var scrapBladeEquipped, rifleEquipped bool
	_ = db.QueryRow("SELECT equipped FROM player_gear WHERE encampment_id = $1 AND gear_key = 'scrap_blade'", campID).Scan(&scrapBladeEquipped)
	_ = db.QueryRow("SELECT equipped FROM player_gear WHERE encampment_id = $1 AND gear_key = 'salvaged_rifle'", campID).Scan(&rifleEquipped)
	if scrapBladeEquipped {
		t.Error("expected scrap_blade to be unequipped after equipping salvaged_rifle (same Weapon slot)")
	}
	if !rifleEquipped {
		t.Error("expected salvaged_rifle to be equipped")
	}
}

func TestGrantRandomDrop_CreditsAnItemOfTheRequestedRarity(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70007)

	def, err := GrantRandomDrop(ctx, db, campID, RarityLegendary)
	if err != nil {
		t.Fatalf("GrantRandomDrop: unexpected error: %v", err)
	}
	if def.Rarity != RarityLegendary {
		t.Errorf("expected a Legendary item, got %v", def.Rarity)
	}

	var quantity int
	if err := db.QueryRow("SELECT quantity FROM player_gear WHERE encampment_id = $1 AND gear_key = $2", campID, def.Key).Scan(&quantity); err != nil {
		t.Fatalf("reading granted gear: %v", err)
	}
	if quantity != 1 {
		t.Errorf("expected quantity 1, got %d", quantity)
	}
}

func TestResourceBonusPctFor_SumsOnlyEquippedGear(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedGearPlayer(t, db, 70008)

	if _, err := db.Exec(`
		INSERT INTO player_gear (encampment_id, gear_key, quantity, equipped) VALUES
		($1, 'scrap_blade', 1, TRUE), ($1, 'patched_vest', 1, TRUE), ($1, 'signal_booster', 1, FALSE)`, campID); err != nil {
		t.Fatalf("seeding gear: %v", err)
	}

	bonus := ResourceBonusPctFor(ctx, db, campID)
	if bonus != 4 {
		t.Errorf("expected 4%% total bonus from equipped-only gear, got %v", bonus)
	}
}
