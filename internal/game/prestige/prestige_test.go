package prestige

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
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database prestige test")
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
	if _, err := db.Exec(`TRUNCATE modules, research_states, resources, encampments, coordinates, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

func seedPrestigePlayer(t *testing.T, db *sql.DB, userID int64, campLevel int) string {
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
		INSERT INTO encampments (user_id, name, coordinate_id, level, extension_lvl) VALUES ($1, 'Prestige Test Camp', $2, $3, 10)
		RETURNING id`, userID, coordID, campLevel).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO resources (encampment_id, scrap, metal) VALUES ($1, 999999, 999999)", campID); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}
	if _, err := db.Exec("INSERT INTO modules (encampment_id, type, level) VALUES ($1, 'scrap_heap', 15)", campID); err != nil {
		t.Fatalf("seeding module: %v", err)
	}
	return campID
}

// TestCheckEligibility_RequiresBothCampLevelAndAllResearchTracks is
// the direct test for NEXT_GENERATION_FEATURES_PLAN.md Phase 4's
// eligibility gate: both camp level AND every one of the 7 research
// tracks must meet their requirement.
func TestCheckEligibility_RequiresBothCampLevelAndAllResearchTracks(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedPrestigePlayer(t, db, 100001, 10) // below campLevelRequirement (25)

	result, err := CheckEligibility(ctx, db, campID)
	if err != nil {
		t.Fatalf("CheckEligibility: unexpected error: %v", err)
	}
	if result.Eligible {
		t.Error("expected a level-10 camp to be ineligible")
	}
	if len(result.Missing) == 0 {
		t.Error("expected a non-empty Missing breakdown")
	}

	// Raise camp level but leave research at the default level 1 -
	// still ineligible.
	if _, err := db.Exec("UPDATE encampments SET level = 25 WHERE id = $1", campID); err != nil {
		t.Fatalf("raising camp level: %v", err)
	}
	result, err = CheckEligibility(ctx, db, campID)
	if err != nil {
		t.Fatalf("CheckEligibility (2nd): unexpected error: %v", err)
	}
	if result.Eligible {
		t.Error("expected the camp to still be ineligible with research tracks at level 1")
	}

	// Raise every research track to the requirement - now eligible.
	if _, err := db.Exec(`
		INSERT INTO research_states (encampment_id, econ_tech_lvl, production_tech_lvl, integrity_tech_lvl, defense_tech_lvl, intel_tech_lvl, speed_tech_lvl, military_tech_lvl)
		VALUES ($1, 20, 20, 20, 20, 20, 20, 20)
		ON CONFLICT (encampment_id) DO UPDATE SET econ_tech_lvl = 20, production_tech_lvl = 20, integrity_tech_lvl = 20, defense_tech_lvl = 20, intel_tech_lvl = 20, speed_tech_lvl = 20, military_tech_lvl = 20`, campID); err != nil {
		t.Fatalf("raising research levels: %v", err)
	}
	result, err = CheckEligibility(ctx, db, campID)
	if err != nil {
		t.Fatalf("CheckEligibility (3rd): unexpected error: %v", err)
	}
	if !result.Eligible {
		t.Errorf("expected the camp to be eligible now, missing: %v", result.Missing)
	}
}

// TestDoPrestige_ResetsEconomyAndGrantsPermanentBonus is the direct
// test for the actual reset: camp level, buildings, research, and
// resources all reset, prestige_level increments, and
// prestige_bonus_pct grows by exactly prestigeBonusPctPerLevel.
func TestDoPrestige_ResetsEconomyAndGrantsPermanentBonus(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedPrestigePlayer(t, db, 100002, 25)
	if _, err := db.Exec(`
		INSERT INTO research_states (encampment_id, econ_tech_lvl, production_tech_lvl, integrity_tech_lvl, defense_tech_lvl, intel_tech_lvl, speed_tech_lvl, military_tech_lvl)
		VALUES ($1, 20, 20, 20, 20, 20, 20, 20)`, campID); err != nil {
		t.Fatalf("seeding eligible research levels: %v", err)
	}

	if err := DoPrestige(ctx, db, campID); err != nil {
		t.Fatalf("DoPrestige: unexpected error: %v", err)
	}

	var level, extensionLvl, prestigeLevel int
	var bonusPct float64
	if err := db.QueryRow("SELECT level, extension_lvl, prestige_level, prestige_bonus_pct FROM encampments WHERE id = $1", campID).Scan(&level, &extensionLvl, &prestigeLevel, &bonusPct); err != nil {
		t.Fatalf("reading encampment: %v", err)
	}
	if level != 1 {
		t.Errorf("expected camp level reset to 1, got %d", level)
	}
	if extensionLvl != 0 {
		t.Errorf("expected extension_lvl reset to 0, got %d", extensionLvl)
	}
	if prestigeLevel != 1 {
		t.Errorf("expected prestige_level to increment to 1, got %d", prestigeLevel)
	}
	if bonusPct != prestigeBonusPctPerLevel {
		t.Errorf("expected prestige_bonus_pct to be exactly %v, got %v", prestigeBonusPctPerLevel, bonusPct)
	}

	var moduleCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM modules WHERE encampment_id = $1", campID).Scan(&moduleCount); err != nil {
		t.Fatalf("counting modules: %v", err)
	}
	if moduleCount != 0 {
		t.Errorf("expected all buildings (modules) to be cleared, got %d remaining", moduleCount)
	}

	var econLvl, militaryLvl int
	if err := db.QueryRow("SELECT econ_tech_lvl, military_tech_lvl FROM research_states WHERE encampment_id = $1", campID).Scan(&econLvl, &militaryLvl); err != nil {
		t.Fatalf("reading research_states: %v", err)
	}
	if econLvl != 1 || militaryLvl != 1 {
		t.Errorf("expected all research tracks reset to 1, got econ=%d military=%d", econLvl, militaryLvl)
	}

	var scrap, metal float64
	if err := db.QueryRow("SELECT scrap, metal FROM resources WHERE encampment_id = $1", campID).Scan(&scrap, &metal); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if scrap != startingResourceBaseline || metal != startingResourceBaseline {
		t.Errorf("expected resources reset to the starting baseline (%v), got scrap=%v metal=%v", startingResourceBaseline, scrap, metal)
	}
}

// TestDoPrestige_RejectsIneligibleCamp confirms DoPrestige refuses to
// run (and changes nothing) for a camp that doesn't meet the
// eligibility gate - the panel already hides the button, but the core
// function itself must not trust that alone.
func TestDoPrestige_RejectsIneligibleCamp(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedPrestigePlayer(t, db, 100003, 10)

	if err := DoPrestige(ctx, db, campID); err == nil {
		t.Error("expected DoPrestige to reject an ineligible camp")
	}

	var level int
	_ = db.QueryRow("SELECT level FROM encampments WHERE id = $1", campID).Scan(&level)
	if level != 10 {
		t.Errorf("expected camp level to remain untouched at 10, got %d", level)
	}
}

// TestDoPrestige_StacksAcrossMultiplePrestiges confirms
// prestige_bonus_pct accumulates (not resets) across repeated
// Prestige cycles - "stacking forever" is the whole point of the
// mechanic.
func TestDoPrestige_StacksAcrossMultiplePrestiges(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID := seedPrestigePlayer(t, db, 100004, 25)
	if _, err := db.Exec(`
		INSERT INTO research_states (encampment_id, econ_tech_lvl, production_tech_lvl, integrity_tech_lvl, defense_tech_lvl, intel_tech_lvl, speed_tech_lvl, military_tech_lvl)
		VALUES ($1, 20, 20, 20, 20, 20, 20, 20)`, campID); err != nil {
		t.Fatalf("seeding eligible research levels: %v", err)
	}

	if err := DoPrestige(ctx, db, campID); err != nil {
		t.Fatalf("first DoPrestige: unexpected error: %v", err)
	}

	// Raise the camp back to eligibility for a second Prestige cycle.
	if _, err := db.Exec("UPDATE encampments SET level = 25 WHERE id = $1", campID); err != nil {
		t.Fatalf("re-raising camp level: %v", err)
	}
	if _, err := db.Exec(`
		UPDATE research_states SET econ_tech_lvl = 20, production_tech_lvl = 20, integrity_tech_lvl = 20, defense_tech_lvl = 20, intel_tech_lvl = 20, speed_tech_lvl = 20, military_tech_lvl = 20
		WHERE encampment_id = $1`, campID); err != nil {
		t.Fatalf("re-raising research levels: %v", err)
	}

	if err := DoPrestige(ctx, db, campID); err != nil {
		t.Fatalf("second DoPrestige: unexpected error: %v", err)
	}

	var prestigeLevel int
	var bonusPct float64
	if err := db.QueryRow("SELECT prestige_level, prestige_bonus_pct FROM encampments WHERE id = $1", campID).Scan(&prestigeLevel, &bonusPct); err != nil {
		t.Fatalf("reading encampment: %v", err)
	}
	if prestigeLevel != 2 {
		t.Errorf("expected prestige_level 2 after two cycles, got %d", prestigeLevel)
	}
	if bonusPct != prestigeBonusPctPerLevel*2 {
		t.Errorf("expected prestige_bonus_pct to be exactly %v after two cycles, got %v", prestigeBonusPctPerLevel*2, bonusPct)
	}
}
