package notifications

import (
	"context"
	"database/sql"
	"testing"
)

// seedBroadcastEncampment creates a user + encampment + coordinate at a
// unique (x, y) in region.
func seedBroadcastEncampment(t *testing.T, db *sql.DB, telegramID int64, x, y int, region string, isAIFaction bool) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $2, 'plains', $3, 'flat')
		ON CONFLICT (x, y) DO UPDATE SET biome = EXCLUDED.biome
		RETURNING id`, x, y, region).Scan(&coordID)
	if err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO encampments (user_id, name, coordinate_id, is_ai_faction) VALUES ($1, $2, $3, $4)`,
		telegramID, "BroadcastTestCamp", coordID, isAIFaction); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
}

// assertNotified checks whether userID has at least one queued
// notification containing message.
func assertNotified(t *testing.T, db *sql.DB, userID int64, want bool, message string) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message = $2", userID, message).Scan(&count); err != nil {
		t.Fatalf("querying notifications for %d: %v", userID, err)
	}
	got := count > 0
	if got != want {
		t.Errorf("user %d: expected notified=%v, got %v (count=%d)", userID, want, got, count)
	}
}

func TestQueueToRegion_OnlyReachesRealPlayersInThatRegion(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	seedBroadcastEncampment(t, db, 8001, 100, 100, "RegionA", false)
	seedBroadcastEncampment(t, db, 8002, 101, 101, "RegionA", false)
	seedBroadcastEncampment(t, db, 8003, 105, 105, "RegionB", false) // different region, must not receive
	seedBroadcastEncampment(t, db, 8004, 102, 102, "RegionA", true)  // AI faction, must not receive

	const message = "weather alert for TestQueueToRegion"
	if err := QueueToRegion(ctx, db, "RegionA", message, "general"); err != nil {
		t.Fatalf("QueueToRegion: %v", err)
	}

	assertNotified(t, db, 8001, true, message)
	assertNotified(t, db, 8002, true, message)
	assertNotified(t, db, 8003, false, message)
	assertNotified(t, db, 8004, false, message)
}

func TestQueueToAllPlayers_ReachesEveryoneExceptAIFactions(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(8101)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(8102)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO users (telegram_id, state) VALUES ($1, 'ai_faction')
		ON CONFLICT (telegram_id) DO UPDATE SET state = 'ai_faction'`, int64(-8103)); err != nil {
		t.Fatalf("seeding AI faction user: %v", err)
	}

	const message = "tax rate changed for TestQueueToAllPlayers"
	if err := QueueToAllPlayers(ctx, db, message, "general"); err != nil {
		t.Fatalf("QueueToAllPlayers: %v", err)
	}

	assertNotified(t, db, 8101, true, message)
	assertNotified(t, db, 8102, true, message)
	assertNotified(t, db, -8103, false, message)
}
