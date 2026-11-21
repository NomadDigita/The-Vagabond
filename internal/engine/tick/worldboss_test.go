package tick

import (
	"context"
	"testing"
	"time"
)

func TestPayoutWorldBossLoot_GauntletGuaranteesDropAndIncrementsKills(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	campID := seedEncampment(t, db, 90101, "Gauntlet Contributor", 50, 50, "TestRegion", false)
	var userID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", campID).Scan(&userID); err != nil {
		t.Fatalf("reading user_id: %v", err)
	}

	var bossID string
	if err := db.QueryRow(`
		INSERT INTO world_bosses (name, emoji, max_hp, current_hp, loot_pool_dollars, tier, is_gauntlet, total_kills)
		VALUES ('Test Gauntlet Boss', '🔥', 1000, 0, 5000, 2, TRUE, 0) RETURNING id`).Scan(&bossID); err != nil {
		t.Fatalf("seeding gauntlet boss: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO world_boss_contributions (boss_id, user_id, encampment_id, damage_dealt) VALUES ($1, $2, $3, 1000)`,
		bossID, userID, campID); err != nil {
		t.Fatalf("seeding contribution: %v", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.payoutWorldBossLoot(ctx, tx, bossID, "Test Gauntlet Boss", 5000, true); err != nil {
		_ = tx.Rollback()
		t.Fatalf("payoutWorldBossLoot: unexpected error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var gearCount int
	if err := db.QueryRow("SELECT COALESCE(SUM(quantity), 0) FROM player_gear WHERE encampment_id = $1", campID).Scan(&gearCount); err != nil {
		t.Fatalf("reading player_gear: %v", err)
	}
	if gearCount != 1 {
		t.Errorf("expected exactly 1 guaranteed gear item from a Gauntlet kill, got %d", gearCount)
	}

	var rarity string
	if err := db.QueryRow(`
		SELECT gear_key FROM player_gear WHERE encampment_id = $1 LIMIT 1`, campID).Scan(&rarity); err == nil {
		if rarity == "" {
			t.Error("expected a real gear_key to be granted")
		}
	}

	var totalKills int
	if err := db.QueryRow("SELECT total_kills FROM world_bosses WHERE id = $1", bossID).Scan(&totalKills); err != nil {
		t.Fatalf("reading total_kills: %v", err)
	}
	if totalKills != 1 {
		t.Errorf("expected total_kills to increment to 1, got %d", totalKills)
	}

	var currentHP, maxHP float64
	if err := db.QueryRow("SELECT current_hp, max_hp FROM world_bosses WHERE id = $1", bossID).Scan(&currentHP, &maxHP); err != nil {
		t.Fatalf("reading boss HP: %v", err)
	}
	if currentHP != maxHP {
		t.Errorf("expected the boss to respawn to full HP (%v), got current_hp=%v", maxHP, currentHP)
	}
}

func TestPayoutWorldBossLoot_NonGauntletDoesNotGuaranteeDrop(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	campID := seedEncampment(t, db, 90102, "Normal Contributor", 51, 51, "TestRegion", false)
	var userID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", campID).Scan(&userID); err != nil {
		t.Fatalf("reading user_id: %v", err)
	}

	anyNoDrop := false
	for i := 0; i < 100 && !anyNoDrop; i++ {
		var bossID string
		if err := db.QueryRow(`
			INSERT INTO world_bosses (name, emoji, max_hp, current_hp, loot_pool_dollars, tier, is_gauntlet)
			VALUES ($1, '👹', 1000, 0, 1000, 1, FALSE) RETURNING id`, "Roll Test Boss").Scan(&bossID); err != nil {
			t.Fatalf("seeding boss: %v", err)
		}
		if _, err := db.Exec(`
			INSERT INTO world_boss_contributions (boss_id, user_id, encampment_id, damage_dealt) VALUES ($1, $2, $3, 1000)`,
			bossID, userID, campID); err != nil {
			t.Fatalf("seeding contribution: %v", err)
		}
		if _, err := db.Exec("DELETE FROM player_gear WHERE encampment_id = $1", campID); err != nil {
			t.Fatalf("clearing gear between rounds: %v", err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.payoutWorldBossLoot(ctx, tx, bossID, "Roll Test Boss", 1000, false); err != nil {
			_ = tx.Rollback()
			t.Fatalf("payoutWorldBossLoot: unexpected error: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}

		var gearCount int
		_ = db.QueryRow("SELECT COALESCE(SUM(quantity), 0) FROM player_gear WHERE encampment_id = $1", campID).Scan(&gearCount)
		if gearCount == 0 {
			anyNoDrop = true
		}
		_, _ = db.Exec("DELETE FROM world_bosses WHERE id = $1", bossID)
	}

	if !anyNoDrop {
		t.Error("expected at least one no-drop result across 100 non-Gauntlet kills (drops are chance-based, not guaranteed)")
	}
}
