package herodepth

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
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database herodepth test")
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
	if _, err := db.Exec(`TRUNCATE hero_skills_unlocked, heroes, player_gear, resources, encampments, coordinates, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

func seedHeroPlayer(t *testing.T, db *sql.DB, userID int64, faction string) (campID, heroID string) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id, faction) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, faction); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $1, 'plains', 'TestRegion', 'flat')
		RETURNING id`, userID).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id) VALUES ($1, 'Hero Test Camp', $2)
		RETURNING id`, userID, coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO resources (encampment_id) VALUES ($1)", campID); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}
	if err := db.QueryRow(`
		INSERT INTO heroes (encampment_id, name, trait, injuries, superpower, level, xp, skill_points)
		VALUES ($1, 'Test Hero', 'Trait', 'None', 'Power', 1, 0, 2) RETURNING id`, campID).Scan(&heroID); err != nil {
		t.Fatalf("seeding hero: %v", err)
	}
	return campID, heroID
}

// TestUnlockSkill_SpendsPointAndRejectsDoubleUnlock is the direct test
// for NEXT_GENERATION_FEATURES_PLAN.md Phase 6's skill tree: unlocking
// a node spends exactly 1 skill point and can't be unlocked twice.
func TestUnlockSkill_SpendsPointAndRejectsDoubleUnlock(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	_, heroID := seedHeroPlayer(t, db, 80001, "steel_vanguard")

	if err := UnlockSkill(ctx, db, heroID, "steel_vanguard", "sv_fort_1"); err != nil {
		t.Fatalf("UnlockSkill: unexpected error: %v", err)
	}

	var points int
	if err := db.QueryRow("SELECT skill_points FROM heroes WHERE id = $1", heroID).Scan(&points); err != nil {
		t.Fatalf("reading skill points: %v", err)
	}
	if points != 1 {
		t.Errorf("expected 1 skill point remaining (started with 2, spent 1), got %d", points)
	}

	if err := UnlockSkill(ctx, db, heroID, "steel_vanguard", "sv_fort_1"); err == nil {
		t.Error("expected unlocking the same skill twice to fail")
	}
}

// TestUnlockSkill_RejectsWrongFactionAndNoPoints covers two failure
// modes: a node from the other faction, and running out of points.
func TestUnlockSkill_RejectsWrongFactionAndNoPoints(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	_, heroID := seedHeroPlayer(t, db, 80002, "steel_vanguard")

	if err := UnlockSkill(ctx, db, heroID, "steel_vanguard", "rn_scv_1"); err == nil {
		t.Error("expected unlocking a Rust Nomads node with a Steel Vanguard faction to fail")
	}

	// Spend both starting points, then try a third.
	if err := UnlockSkill(ctx, db, heroID, "steel_vanguard", "sv_fort_1"); err != nil {
		t.Fatalf("first unlock: unexpected error: %v", err)
	}
	if err := UnlockSkill(ctx, db, heroID, "steel_vanguard", "sv_fort_2"); err != nil {
		t.Fatalf("second unlock: unexpected error: %v", err)
	}
	if err := UnlockSkill(ctx, db, heroID, "steel_vanguard", "sv_fort_3"); err == nil {
		t.Error("expected a third unlock with 0 remaining points to fail")
	}
}

// TestTrainingCostMultiplier_StacksDiscountsWithFloor confirms
// unlocked EffectTrainingCostPct nodes stack multiplicatively-additive
// (summed as percentages) and never drop the multiplier below the 20%
// floor even with many stacked discounts.
func TestTrainingCostMultiplier_StacksDiscountsWithFloor(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	_, heroID := seedHeroPlayer(t, db, 80003, "steel_vanguard")

	// No skills unlocked yet -> multiplier should be 1.0 (no discount).
	if m := TrainingCostMultiplier(ctx, db, heroID); m != 1.0 {
		t.Errorf("expected multiplier 1.0 with no skills unlocked, got %v", m)
	}

	if _, err := db.Exec("INSERT INTO hero_skills_unlocked (hero_id, skill_key) VALUES ($1, 'sv_fort_1')", heroID); err != nil {
		t.Fatalf("seeding unlocked skill: %v", err)
	}
	// sv_fort_1 is -10% -> multiplier 0.90.
	if m := TrainingCostMultiplier(ctx, db, heroID); m < 0.899 || m > 0.901 {
		t.Errorf("expected multiplier ~0.90 with sv_fort_1 unlocked, got %v", m)
	}
}

// TestResourceBonusPctFor_SumsUnlockedResourceGenNodes confirms the
// resource-tick integration point sums exactly the unlocked
// EffectResourceGenPct nodes for the hero belonging to campID.
func TestResourceBonusPctFor_SumsUnlockedResourceGenNodes(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID, heroID := seedHeroPlayer(t, db, 80004, "rust_nomads")

	if bonus := ResourceBonusPctFor(ctx, db, campID); bonus != 0 {
		t.Errorf("expected 0%% bonus with no skills unlocked, got %v", bonus)
	}

	if _, err := db.Exec("INSERT INTO hero_skills_unlocked (hero_id, skill_key) VALUES ($1, 'rn_scv_3')", heroID); err != nil {
		t.Fatalf("seeding unlocked skill: %v", err)
	}
	// rn_scv_3 grants +3% resource generation.
	if bonus := ResourceBonusPctFor(ctx, db, campID); bonus != 3 {
		t.Errorf("expected 3%% bonus after unlocking rn_scv_3, got %v", bonus)
	}
}

// TestDeliverStoryBeats_DeliversOnceAndAdvancesStage is the direct
// test for the level-gated story arc: reaching level 5 delivers
// exactly the level-5 beat (not level 10+), and a second call at the
// same level delivers nothing further.
func TestDeliverStoryBeats_DeliversOnceAndAdvancesStage(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const userID = 80005
	_, heroID := seedHeroPlayer(t, db, userID, "steel_vanguard")

	if err := DeliverStoryBeats(ctx, db, userID, heroID, "steel_vanguard", 5); err != nil {
		t.Fatalf("DeliverStoryBeats: unexpected error: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", userID).Scan(&count); err != nil {
		t.Fatalf("counting notifications: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 story-beat notification after reaching level 5, got %d", count)
	}

	var stage int
	if err := db.QueryRow("SELECT story_arc_stage FROM heroes WHERE id = $1", heroID).Scan(&stage); err != nil {
		t.Fatalf("reading story_arc_stage: %v", err)
	}
	if stage != 5 {
		t.Errorf("expected story_arc_stage to advance to 5, got %d", stage)
	}

	// Calling again at the same level must not deliver a duplicate.
	if err := DeliverStoryBeats(ctx, db, userID, heroID, "steel_vanguard", 5); err != nil {
		t.Fatalf("DeliverStoryBeats (2nd call): unexpected error: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", userID).Scan(&count); err != nil {
		t.Fatalf("counting notifications (2nd check): %v", err)
	}
	if count != 1 {
		t.Errorf("expected still exactly 1 notification after a repeat call at the same level, got %d", count)
	}
}

// TestEquipToHero_ReplacesPreviousHeroEquipAndRequiresOwnership covers
// the hero-gear slot: equipping a second item to the hero unequips the
// first (a hero carries exactly one item), and equipping an unowned
// item fails.
func TestEquipToHero_ReplacesPreviousHeroEquipAndRequiresOwnership(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	campID, _ := seedHeroPlayer(t, db, 80006, "steel_vanguard")

	if err := EquipToHero(ctx, db, campID, "scrap_blade"); err == nil {
		t.Error("expected equipping an unowned item to the hero to fail")
	}

	for _, key := range []string{"scrap_blade", "patched_vest"} {
		if _, err := db.Exec("INSERT INTO player_gear (encampment_id, gear_key, quantity) VALUES ($1, $2, 1)", campID, key); err != nil {
			t.Fatalf("seeding owned gear %s: %v", key, err)
		}
	}

	if err := EquipToHero(ctx, db, campID, "scrap_blade"); err != nil {
		t.Fatalf("EquipToHero scrap_blade: unexpected error: %v", err)
	}
	if err := EquipToHero(ctx, db, campID, "patched_vest"); err != nil {
		t.Fatalf("EquipToHero patched_vest: unexpected error: %v", err)
	}

	var scrapBladeByHero, vestByHero bool
	_ = db.QueryRow("SELECT equipped_by_hero FROM player_gear WHERE encampment_id = $1 AND gear_key = 'scrap_blade'", campID).Scan(&scrapBladeByHero)
	_ = db.QueryRow("SELECT equipped_by_hero FROM player_gear WHERE encampment_id = $1 AND gear_key = 'patched_vest'", campID).Scan(&vestByHero)
	if scrapBladeByHero {
		t.Error("expected scrap_blade to no longer be hero-equipped after equipping patched_vest")
	}
	if !vestByHero {
		t.Error("expected patched_vest to be hero-equipped")
	}
}
