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
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ('Human-Led Clan', 5003) RETURNING id").Scan(&clanID); err != nil {
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

	// 500 iterations turned out to be statistically marginal: only the
	// "found" branch of the 1%-chance/tick roll can ever succeed in this
	// test (there's no other federation for the 50% "join" branch to
	// find), so the real per-tick success probability is ~0.5%, giving
	// 500 tries roughly an 8% false-failure rate - caught by a shuffled,
	// repeated-count test run, not a rare fluke worth ignoring. 3000
	// iterations brings the false-failure rate below one in a million.
	joined := false
	for i := 0; i < 3000 && !joined; i++ {
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
		t.Fatal("expected the AI clan Leader's clan to end up in a federation at least once across 3000 ticks")
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

// TestGrowAICivilizationsCanDisbandClanAsLeader is the direct test for
// the AI clan leave/disband gap the project owner flagged 2026-08-09:
// a Leader-role AI faction must eventually disband a clan it leads
// (DELETE FROM clans, cascading to user_clans) rather than being
// permanently stuck leading it forever.
func TestGrowAICivilizationsCanDisbandClanAsLeader(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2030, "Faction Sigma", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	myClanID := seedAIClanLeader(t, db, factionUserID, "AI Clan Sigma")

	disbanded := false
	for i := 0; i < 1500 && !disbanded; i++ {
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
		var stillExists bool
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM clans WHERE id = $1)", myClanID).Scan(&stillExists)
		disbanded = !stillExists
	}
	if !disbanded {
		t.Fatal("expected the AI clan Leader to disband its clan at least once across 1500 ticks")
	}

	var memberRowExists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", factionUserID).Scan(&memberRowExists)
	if memberRowExists {
		t.Error("expected the disbanded clan's cascade to also remove the AI Leader's own user_clans row")
	}
}

// TestGrowAICivilizationsCanDisbandClanAsLeader_NotifiesRealMembers
// confirms a real human member of an AI-led clan is notified when
// their AI Leader disbands it - the clan disappearing out from under
// them should never be silent.
func TestGrowAICivilizationsCanDisbandClanAsLeader_NotifiesRealMembers(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2031, "Faction Tau", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	myClanID := seedAIClanLeader(t, db, factionUserID, "AI Clan Tau")

	const humanMemberID = 5004
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", humanMemberID); err != nil {
		t.Fatalf("seeding human member user: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Member')", humanMemberID, myClanID); err != nil {
		t.Fatalf("seeding human member: %v", err)
	}

	disbanded := false
	for i := 0; i < 1500 && !disbanded; i++ {
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
		var stillExists bool
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM clans WHERE id = $1)", myClanID).Scan(&stillExists)
		disbanded = !stillExists
	}
	if !disbanded {
		t.Fatal("expected the AI clan Leader to disband its clan at least once across 1500 ticks")
	}

	var notified bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM notifications WHERE user_id = $1 AND message LIKE '%DISBANDED%')", humanMemberID).Scan(&notified)
	if !notified {
		t.Error("expected the real human member to be notified that their AI-led clan was disbanded")
	}
}

// TestGrowAICivilizationsCanLeaveClanAsMember confirms a rank-and-file
// (non-Leader) AI clan member eventually leaves without touching the
// clan itself - the clan should survive with the human Leader still in
// charge.
func TestGrowAICivilizationsCanLeaveClanAsMember(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2032, "Faction Upsilon", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES (5005) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("seeding human leader: %v", err)
	}
	var clanID string
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ('Human-Led Clan Upsilon', 5005) RETURNING id").Scan(&clanID); err != nil {
		t.Fatalf("seeding clan: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Member')", factionUserID, clanID); err != nil {
		t.Fatalf("seeding AI as a rank-and-file member: %v", err)
	}

	left := false
	for i := 0; i < 1500 && !left; i++ {
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
		var stillMember bool
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", factionUserID).Scan(&stillMember)
		left = !stillMember
	}
	if !left {
		t.Fatal("expected the rank-and-file AI member to leave its clan at least once across 1500 ticks")
	}

	var clanStillExists bool
	_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM clans WHERE id = $1)", clanID).Scan(&clanStillExists)
	if !clanStillExists {
		t.Error("expected the clan itself to survive a rank-and-file member leaving (only the Leader leaving should disband it)")
	}
}
