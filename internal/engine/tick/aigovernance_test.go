package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// seedAIClanLeader creates a clan led by the given AI faction's user_id,
// returning the clan id - a small helper shared by the clan-war and
// federation tests below, both of which require the faction to already
// lead a clan (mirrors user_clans.role = 'Leader', the same gate
// HandleDeclareClanWarCallback and HandleFoundFederation/HandleJoinFederation
// use for humans).
func seedAIClanLeader(t *testing.T, db *sql.DB, factionUserID int64, clanName string) string {
	t.Helper()
	var clanID string
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ($1, $2) RETURNING id", clanName, factionUserID).Scan(&clanID); err != nil {
		t.Fatalf("seeding clan: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Leader')", factionUserID, clanID); err != nil {
		t.Fatalf("seeding clan leadership: %v", err)
	}
	return clanID
}

// TestGrowAICivilizationsCanDeclareClanWar verifies a Leader-role AI
// faction eventually declares war on an available rival clan, mirroring
// HandleDeclareClanWarCallback's exact mechanics.
func TestGrowAICivilizationsCanDeclareClanWar(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2028, "Faction Pi", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	myClanID := seedAIClanLeader(t, db, factionUserID, "AI Clan Pi")

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES (5002) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("seeding rival leader: %v", err)
	}
	if _, err := db.Exec("INSERT INTO clans (name, leader_id) VALUES ('Rival Clan', 5002)"); err != nil {
		t.Fatalf("seeding rival clan: %v", err)
	}

	declared := false
	for i := 0; i < 300 && !declared; i++ {
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
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM clan_wars WHERE clan_a_id = $1 AND status = 'active')", myClanID).Scan(&declared)
	}
	if !declared {
		t.Fatal("expected the AI clan Leader to declare a war at least once across 300 ticks")
	}
}

// TestGrowAICivilizationsWontDeclareWarWhenNotLeader verifies a
// rank-and-file AI clan member (not the Leader) never declares a war -
// mirrors HandleDeclareClanWarCallback's "Leaders only" guard.
func TestGrowAICivilizationsWontDeclareWarWhenNotLeader(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2029, "Faction Rho", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES (5003) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("seeding human leader: %v", err)
	}
	var clanID string
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ('Human-Led Clan', 5003) RETURNING id", ).Scan(&clanID); err != nil {
		t.Fatalf("seeding clan: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Member')", factionUserID, clanID); err != nil {
		t.Fatalf("seeding AI as a rank-and-file member: %v", err)
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

	var warCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM clan_wars WHERE clan_a_id = $1 OR clan_b_id = $1", clanID).Scan(&warCount)
	if warCount != 0 {
		t.Errorf("expected a non-Leader AI member to never declare a war, but %d war(s) were declared", warCount)
	}
}

// TestGrowAICivilizationsCanFoundOrJoinFederation verifies a Leader-role
// AI faction with enough Crystal eventually gets its clan into a
// federation - either by founding one (HandleFoundFederation's mechanics)
// or joining an existing one (HandleJoinFederation's mechanics).
func TestGrowAICivilizationsCanFoundOrJoinFederation(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2030, "Faction Sigma", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	if _, err := db.Exec("UPDATE resources SET crystal = 10000 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding the AI faction with crystal: %v", err)
	}
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	myClanID := seedAIClanLeader(t, db, factionUserID, "AI Clan Sigma")

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
		// Replenish crystal each round so the founding path stays
		// available even after a listing/buying roll spent some.
		if _, err := db.Exec("UPDATE resources SET crystal = 10000 WHERE encampment_id = $1", faction); err != nil {
			t.Fatalf("replenishing crystal: %v", err)
		}
		var fedID sql.NullString
		_ = db.QueryRow("SELECT federation_id FROM clans WHERE id = $1", myClanID).Scan(&fedID)
		joined = fedID.Valid
	}
	if !joined {
		t.Fatal("expected the AI clan Leader's clan to end up in a federation at least once across 500 ticks")
	}
}

// TestGrowAICivilizationsCanQueueForArena verifies a funded AI faction
// eventually joins the arena queue, mirroring HandleJoinQueueCallback's
// exact mechanics (entry fee debited, arena_queue row inserted).
func TestGrowAICivilizationsCanQueueForArena(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2031, "Faction Tau", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	if _, err := db.Exec("UPDATE resources SET dollars = 500 WHERE encampment_id = $1", faction); err != nil {
		t.Fatalf("funding the AI faction: %v", err)
	}

	queued := false
	for i := 0; i < 300 && !queued; i++ {
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
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM arena_queue WHERE user_id = $1)", factionUserID).Scan(&queued)
	}
	if !queued {
		t.Fatal("expected the AI faction to join the arena queue at least once across 300 ticks")
	}

	var bracket string
	_ = db.QueryRow("SELECT bracket FROM arena_queue WHERE user_id = $1", factionUserID).Scan(&bracket)
	if bracket != "solo" {
		t.Errorf("expected the AI faction to queue in the 'solo' bracket, got %q", bracket)
	}
}
