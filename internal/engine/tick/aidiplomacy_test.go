package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestGrowAICivilizationsCanProposeAndAcceptPacts verifies both
// diplomacy paths: a Leader-role AI faction with a pending proposal
// addressed to it eventually responds (mirrors
// HandleDiplomacyRespondCallback), and, separately, a Leader-role AI
// faction with no pending proposal eventually proposes a new pact to an
// unrelated clan (mirrors proposePact/HandleProposeAlliance/HandleProposeNAP).
func TestGrowAICivilizationsCanProposeAndAcceptPacts(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	// --- Responding to a pending proposal ---
	faction := seedEncampment(t, db, 2032, "Faction Upsilon", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 5)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	myClanID := seedAIClanLeader(t, db, factionUserID, "AI Clan Upsilon")

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES (5004) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("seeding proposer: %v", err)
	}
	var proposerClanID string
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ('Proposer Clan', 5004) RETURNING id").Scan(&proposerClanID); err != nil {
		t.Fatalf("seeding proposer clan: %v", err)
	}
	var pactID string
	if err := db.QueryRow(`
		INSERT INTO clan_diplomacy (clan_a_id, clan_b_id, pact_type, proposed_by)
		VALUES ($1, $2, 'alliance', 5004) RETURNING id`, proposerClanID, myClanID).Scan(&pactID); err != nil {
		t.Fatalf("seeding a pending proposal: %v", err)
	}

	responded := false
	for i := 0; i < 300 && !responded; i++ {
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
		var status string
		_ = db.QueryRow("SELECT status FROM clan_diplomacy WHERE id = $1", pactID).Scan(&status)
		responded = status != "pending"
	}
	if !responded {
		t.Fatal("expected the AI Leader to respond to the pending proposal at least once across 300 ticks")
	}

	// --- Proposing a new pact ---
	factionB := seedEncampment(t, db, 2033, "Faction Phi", 1, 0, "TestRegion", true)
	setAILevel(t, db, factionB, 5)
	var factionBUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", factionB).Scan(&factionBUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	myClanBID := seedAIClanLeader(t, db, factionBUserID, "AI Clan Phi")

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES (5005) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("seeding a target leader: %v", err)
	}
	if _, err := db.Exec("INSERT INTO clans (name, leader_id) VALUES ('Unrelated Clan', 5005)"); err != nil {
		t.Fatalf("seeding an unrelated clan: %v", err)
	}

	proposed := false
	for i := 0; i < 300 && !proposed; i++ {
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
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM clan_diplomacy WHERE clan_a_id = $1 OR clan_b_id = $1)", myClanBID).Scan(&proposed)
	}
	if !proposed {
		t.Fatal("expected the AI Leader to propose a new pact at least once across 300 ticks")
	}
}

// TestPickFairAIRaidTargetRespectsActivePact verifies a target whose
// clan has an active pact with the attacking faction's clan is excluded
// from raid selection - the same rule HasActivePact enforces for
// human-launched raids.
func TestPickFairAIRaidTargetRespectsActivePact(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	faction := seedEncampment(t, db, 2034, "Faction Chi", 0, 0, "TestRegion", true)
	setAILevel(t, db, faction, 10)
	var factionUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", faction).Scan(&factionUserID); err != nil {
		t.Fatalf("looking up faction user_id: %v", err)
	}
	myClanID := seedAIClanLeader(t, db, factionUserID, "AI Clan Chi")

	protectedTarget := seedEncampment(t, db, 2035, "Protected Player", 1, 0, "TestRegion", false)
	setAILevel(t, db, protectedTarget, 11) // in-band otherwise
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES (5006) ON CONFLICT DO NOTHING"); err != nil {
		t.Fatalf("seeding protected target's clan leader: %v", err)
	}
	var protectedClanID string
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ('Protected Clan', 5006) RETURNING id").Scan(&protectedClanID); err != nil {
		t.Fatalf("seeding protected clan: %v", err)
	}
	var protectedUserID int64
	if err := db.QueryRow("SELECT user_id FROM encampments WHERE id = $1", protectedTarget).Scan(&protectedUserID); err != nil {
		t.Fatalf("looking up protected target user_id: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Member')", protectedUserID, protectedClanID); err != nil {
		t.Fatalf("seeding protected target's clan membership: %v", err)
	}
	if _, err := db.Exec("INSERT INTO clan_diplomacy (clan_a_id, clan_b_id, pact_type, proposed_by, status) VALUES ($1, $2, 'alliance', 5006, 'active')", myClanID, protectedClanID); err != nil {
		t.Fatalf("seeding an active pact: %v", err)
	}

	openTarget := seedEncampment(t, db, 2036, "Open Player", 2, 0, "TestRegion", false)
	setAILevel(t, db, openTarget, 11)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	for _, target := range []string{protectedTarget, openTarget} {
		if _, err := tx.Exec("INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'ai_scout')", faction, target); err != nil {
			t.Fatalf("seeding discovery: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	seenOpen := false
	for i := 0; i < 50; i++ {
		tx2, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		target, err := e.pickFairAIRaidTarget(ctx, tx2, faction, 10, sql.NullString{String: myClanID, Valid: true})
		_ = tx2.Rollback()
		if err != nil {
			t.Fatalf("pickFairAIRaidTarget: %v", err)
		}
		if target == nil {
			continue
		}
		if target.id == protectedTarget {
			t.Fatalf("pickFairAIRaidTarget returned a target protected by an active pact - pact was not respected")
		}
		if target.id == openTarget {
			seenOpen = true
		}
	}
	if !seenOpen {
		t.Error("expected pickFairAIRaidTarget to still return the unprotected target across 50 attempts")
	}
}
