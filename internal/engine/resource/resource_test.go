package resource

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
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database resource test")
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
	if _, err := db.Exec(`TRUNCATE world_events, modules, resources, encampments, coordinates, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

// seedResourceCamp creates a camp with a Scrap Heap, Generator, Metal
// Mine, and Crystal Mine all at level 1 (so every passive generation
// line RunResourcePass touches produces a clean, non-zero, easily
// hand-verified amount) in the given region.
func seedResourceCamp(t *testing.T, db *sql.DB, telegramID int64, region string) string {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $1, 'plains', $2, 'flat')
		RETURNING id`, telegramID, region).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id) VALUES ($1, 'Bloom Test Camp', $2)
		RETURNING id`, telegramID, coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO resources (encampment_id, scrap, metal, crystal) VALUES ($1, 0, 0, 0)", campID); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}
	for _, moduleType := range []string{"scrap_heap", "generator", "metal_mine", "crystal_mine"} {
		if _, err := db.Exec("INSERT INTO modules (encampment_id, type, level) VALUES ($1, $2, 1)", campID, moduleType); err != nil {
			t.Fatalf("seeding %s module: %v", moduleType, err)
		}
	}
	return campID
}

// TestRunResourcePass_BloomBoostsPassiveGenerationBy15Percent is the
// direct test for RARE_WORLD_FEATURES_PLAN.md Phase 2.3: a continent
// with an active "bloom" event should see every passively-generated
// resource line (Scrap/Metal/Crystal here - the ones a level-1 camp
// with no loan/troops actually produces a clean, comparable amount of)
// come out to exactly 1.15x what an identical camp in a nominal-weather
// continent produces in the same pass.
func TestRunResourcePass_BloomBoostsPassiveGenerationBy15Percent(t *testing.T) {
	db := testDB(t)
	p := NewProcessor(db)
	ctx := context.Background()

	bloomCamp := seedResourceCamp(t, db, 61001, "BloomRegion")
	nominalCamp := seedResourceCamp(t, db, 61002, "NominalRegion")

	if _, err := db.Exec(`
		INSERT INTO world_events (title, event_type, continent, starts_at, expires_at)
		VALUES ('Test Bloom', 'bloom', 'BloomRegion', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '2 hours')`); err != nil {
		t.Fatalf("seeding bloom world event: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := p.RunResourcePass(ctx, tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("RunResourcePass: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var bloomScrap, bloomMetal, bloomCrystal float64
	if err := db.QueryRow("SELECT scrap, metal, crystal FROM resources WHERE encampment_id = $1", bloomCamp).Scan(&bloomScrap, &bloomMetal, &bloomCrystal); err != nil {
		t.Fatalf("reading bloom camp resources: %v", err)
	}
	var nominalScrap, nominalMetal, nominalCrystal float64
	if err := db.QueryRow("SELECT scrap, metal, crystal FROM resources WHERE encampment_id = $1", nominalCamp).Scan(&nominalScrap, &nominalMetal, &nominalCrystal); err != nil {
		t.Fatalf("reading nominal camp resources: %v", err)
	}

	const tolerance = 0.001
	assertRatio := func(label string, bloomVal, nominalVal float64) {
		if nominalVal == 0 {
			t.Fatalf("%s: nominal camp generated exactly 0, can't compute a ratio - test fixture is broken", label)
		}
		ratio := bloomVal / nominalVal
		if ratio < 1.15-tolerance || ratio > 1.15+tolerance {
			t.Errorf("%s: expected bloom camp to generate 1.15x the nominal camp (nominal=%.4f, bloom=%.4f, ratio=%.4f)", label, nominalVal, bloomVal, ratio)
		}
	}
	assertRatio("scrap", bloomScrap, nominalScrap)
	assertRatio("metal", bloomMetal, nominalMetal)
	assertRatio("crystal", bloomCrystal, nominalCrystal)
}
