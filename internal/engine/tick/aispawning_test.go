package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestSpawnNewAIFactionsCreatesAFullyFunctionalEncampment(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	var beforeCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&beforeCount)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx); err != nil {
		t.Fatalf("spawnNewAIFactions: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var afterCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&afterCount)
	if afterCount != beforeCount+1 {
		t.Fatalf("expected exactly one new AI faction, went from %d to %d", beforeCount, afterCount)
	}

	var campID, aiFactionKey string
	var level int
	var isAI bool
	if err := db.QueryRow(`
		SELECT id, ai_faction_key, level, is_ai_faction FROM encampments
		WHERE is_ai_faction = TRUE ORDER BY established_at DESC LIMIT 1`).
		Scan(&campID, &aiFactionKey, &level, &isAI); err != nil {
		t.Fatalf("querying spawned faction: %v", err)
	}
	if !isAI {
		t.Error("expected the spawned encampment to be flagged is_ai_faction")
	}
	if level != aiFactionSpawnStartingLevel {
		t.Errorf("expected the spawned faction at the deliberately-weak starting level %d, got %d", aiFactionSpawnStartingLevel, level)
	}
	if aiFactionKey == "" {
		t.Error("expected a non-empty ai_faction_key")
	}

	var resourceCount, garrisonCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM resources WHERE encampment_id = $1", campID).Scan(&resourceCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&garrisonCount)
	if resourceCount != 1 {
		t.Errorf("expected exactly one resources row for the spawned faction, got %d", resourceCount)
	}
	if garrisonCount != 1 {
		t.Errorf("expected exactly one workshop_inventory row for the spawned faction, got %d", garrisonCount)
	}

	var lastSpawnAt sql.NullTime
	var spawnedCount int
	_ = db.QueryRow("SELECT last_ai_spawn_at, ai_factions_spawned_count FROM world_state WHERE id = 1").Scan(&lastSpawnAt, &spawnedCount)
	if !lastSpawnAt.Valid {
		t.Error("expected world_state.last_ai_spawn_at to be set after a spawn")
	}
	if spawnedCount != 1 {
		t.Errorf("expected world_state.ai_factions_spawned_count = 1, got %d", spawnedCount)
	}

	var newsCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM world_news WHERE headline LIKE '%NEW SETTLEMENT DETECTED%'").Scan(&newsCount)
	if newsCount != 1 {
		t.Errorf("expected exactly one 'NEW SETTLEMENT DETECTED' world_news headline, got %d", newsCount)
	}
}

func TestSpawnNewAIFactionsRespectsCadence(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx); err != nil {
		t.Fatalf("first spawnNewAIFactions: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var countAfterFirst int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&countAfterFirst)

	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx2); err != nil {
		t.Fatalf("second (should be gated) spawnNewAIFactions: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var countAfterSecond int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&countAfterSecond)
	if countAfterSecond != countAfterFirst {
		t.Errorf("expected the cadence gate to block a second spawn immediately after the first, went from %d to %d", countAfterFirst, countAfterSecond)
	}

	if _, err := db.Exec("UPDATE world_state SET last_ai_spawn_at = CURRENT_TIMESTAMP - INTERVAL '1 day' WHERE id = 1"); err != nil {
		t.Fatalf("forcing cadence elapsed: %v", err)
	}
	tx3, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := e.spawnNewAIFactions(ctx, tx3); err != nil {
		t.Fatalf("third spawnNewAIFactions: %v", err)
	}
	if err := tx3.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var countAfterThird int
	_ = db.QueryRow("SELECT COUNT(*) FROM encampments WHERE is_ai_faction = TRUE").Scan(&countAfterThird)
	if countAfterThird != countAfterFirst+1 {
		t.Errorf("expected a spawn once the cadence had elapsed, went from %d to %d", countAfterFirst, countAfterThird)
	}
}

func TestGrowAICivilizationsCanListSurplusOnExchange(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2023, "Faction Mu", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET crystal = 100 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("seeding crystal surplus: %v", err)
	}

	for i := 0; i < 200; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.growAICivilizations(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("growAICivilizations: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if _, err := db.Exec("UPDATE resources SET crystal = 100 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("replenishing crystal: %v", err)
		}

		var listingCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM market_exchange WHERE seller_id = $1 AND item_type = 'crystal'", faction).Scan(&listingCount)
		if listingCount > 0 {
			return
		}
	}
	t.Error("expected the AI faction to list crystal on the exchange at least once across 200 ticks at a 3% chance per tick")
}

func TestGrowAICivilizationsCanBuyAvailableListing(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2024, "Faction Nu", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET dollars = 500 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding the AI faction: %v", err)
	}

	seller := seedEncampment(t, db, 2025, "Human Seller", 1, 0, "TestRegion", false)
	var listingID string
	if err := db.QueryRow(`
		INSERT INTO market_exchange (seller_id, item_type, quantity, price_dollars, is_sold)
		VALUES ($1, 'metal', 50, 150.0, FALSE) RETURNING id`, seller).Scan(&listingID); err != nil {
		t.Fatalf("seeding a listing: %v", err)
	}

	bought := false
	for i := 0; i < 300 && !bought; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.growAICivilizations(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("growAICivilizations: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		var isSold bool
		_ = db.QueryRow("SELECT is_sold FROM market_exchange WHERE id = $1", listingID).Scan(&isSold)
		bought = isSold
	}
	if !bought {
		t.Fatal("expected the AI faction to buy the available listing at least once across 300 ticks")
	}

	var sellerDollars, buyerMetal float64
	_ = db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", seller).Scan(&sellerDollars)
	_ = db.QueryRow("SELECT metal FROM resources WHERE encampment_id = $1", faction).Scan(&buyerMetal)
	if sellerDollars < 150.0 {
		t.Errorf("expected the seller to have received at least $150, got $%.2f", sellerDollars)
	}
	if buyerMetal < 50.0 {
		t.Errorf("expected the AI buyer to have received at least 50 metal, got %.2f", buyerMetal)
	}
}

func TestGrowAICivilizationsWontBuyItsOwnListing(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2026, "Faction Xi", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET dollars = 500 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding the AI faction: %v", err)
	}
	var listingID string
	if err := db.QueryRow(`
		INSERT INTO market_exchange (seller_id, item_type, quantity, price_dollars, is_sold)
		VALUES ($1, 'metal', 50, 150.0, FALSE) RETURNING id`, faction).Scan(&listingID); err != nil {
		t.Fatalf("seeding the faction's own listing: %v", err)
	}

	for i := 0; i < 100; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.growAICivilizations(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("growAICivilizations: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}

	var isSold bool
	_ = db.QueryRow("SELECT is_sold FROM market_exchange WHERE id = $1", listingID).Scan(&isSold)
	if isSold {
		t.Error("expected the AI faction's own listing to never be bought by itself, even across 100 ticks")
	}
}

func TestGrowAICivilizationsCanFoundOrJoinAClan(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2027, "Faction Omicron", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES (5001) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("seeding human leader: %v", err)
	}
	if _, err := db.Exec("INSERT INTO clans (name, leader_id, recruiting) VALUES ('Human Clan', 5001, TRUE)"); err != nil {
		t.Fatalf("seeding a recruiting clan: %v", err)
	}

	joined := false
	for i := 0; i < 500 && !joined; i++ {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		if err := e.growAICivilizations(ctx, tx); err != nil {
			_ = tx.Rollback()
			t.Fatalf("growAICivilizations: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		var inClan, hasApplication bool
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", factionUserID).Scan(&inClan)
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM clan_applications WHERE user_id = $1)", factionUserID).Scan(&hasApplication)
		joined = inClan || hasApplication
	}
	if !joined {
		t.Fatal("expected the AI faction to either found or apply to a clan at least once across 500 ticks")
	}

	var ownClanCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM clans WHERE leader_id = $1", factionUserID).Scan(&ownClanCount)
	var appCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM clan_applications WHERE user_id = $1 AND status = 'pending'", factionUserID).Scan(&appCount)
	if ownClanCount == 0 && appCount == 0 {
		t.Error("expected either a founded clan (as leader) or a pending application, got neither")
	}
	var inHumanClan bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_clans uc JOIN clans c ON c.id = uc.clan_id WHERE uc.user_id = $1 AND c.leader_id = 5001)", factionUserID).Scan(&inHumanClan)
	if inHumanClan {
		t.Error("expected the AI faction to never be auto-joined into the human's clan without the leader accepting an application")
	}
}
