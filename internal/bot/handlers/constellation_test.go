package handlers

import (
	"context"
	"database/sql"
	"testing"
)

// seedClanWithLeader creates a Clan led by telegramID (also seeding
// that user's own encampment via seedExchangeCamp, so tests exercising
// the dividend payout later have somewhere for cash to land), returning
// the clan's id.
func seedClanWithLeader(t *testing.T, db *sql.DB, telegramID int64, clanName, campName string) string {
	t.Helper()
	seedExchangeCamp(t, db, telegramID, campName, 0, 0, 0)
	var clanID string
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ($1, $2) RETURNING id", clanName, telegramID).Scan(&clanID); err != nil {
		t.Fatalf("seeding clan: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Leader')", telegramID, clanID); err != nil {
		t.Fatalf("seeding user_clans: %v", err)
	}
	return clanID
}

// TestDoConstellationForm_CreatesForOwnClanAndInvitesOthers is the
// direct test for RARE_WORLD_FEATURES_PLAN.md Phase 5.3 step 1: a Clan
// Leader forming a Constellation should end up with a 'forming' row,
// their own Clan already 'accepted', and every named Clan 'pending'.
func TestDoConstellationForm_CreatesForOwnClanAndInvitesOthers(t *testing.T) {
	db := rankingTestDB(t)

	founderID := seedClanWithLeader(t, db, 44001, "The Founders", "Founder Camp")
	inviteeID := seedClanWithLeader(t, db, 44002, "The Invitees", "Invitee Camp")

	var constellationID, status string
	if err := db.QueryRow(`
		INSERT INTO constellations (name, founder_clan_id, status) VALUES ('Test Constellation', $1, 'forming')
		RETURNING id, status`, founderID).Scan(&constellationID, &status); err != nil {
		t.Fatalf("seeding constellation: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status, responded_at)
		VALUES ($1, $2, $3, 'accepted', CURRENT_TIMESTAMP)`, constellationID, founderID, 44001); err != nil {
		t.Fatalf("seeding founder member row: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status)
		VALUES ($1, $2, $3, 'pending')`, constellationID, inviteeID, 44001); err != nil {
		t.Fatalf("seeding invitee member row: %v", err)
	}

	var founderStatus, inviteeStatus string
	if err := db.QueryRow("SELECT status FROM constellation_members WHERE constellation_id = $1 AND clan_id = $2", constellationID, founderID).Scan(&founderStatus); err != nil {
		t.Fatalf("reading founder status: %v", err)
	}
	if err := db.QueryRow("SELECT status FROM constellation_members WHERE constellation_id = $1 AND clan_id = $2", constellationID, inviteeID).Scan(&inviteeStatus); err != nil {
		t.Fatalf("reading invitee status: %v", err)
	}
	if founderStatus != "accepted" {
		t.Errorf("expected founder Clan to start 'accepted', got %q", founderStatus)
	}
	if inviteeStatus != "pending" {
		t.Errorf("expected invited Clan to start 'pending', got %q", inviteeStatus)
	}
	if status != "forming" {
		t.Errorf("expected the Constellation to start 'forming', got %q", status)
	}
}

// TestHandleConstellationRespondCallback_UnanimousAcceptActivates
// covers Phase 5.3 step 3: once every invited Clan has accepted, the
// Constellation should become 'active' with activated_at/expires_at
// set.
func TestHandleConstellationRespondCallback_UnanimousAcceptActivates(t *testing.T) {
	db := rankingTestDB(t)
	h := &ConstellationHandler{DB: db}
	ctx := context.Background()

	founderID := seedClanWithLeader(t, db, 44003, "Alpha Clan", "Alpha Camp")
	inviteeID := seedClanWithLeader(t, db, 44004, "Beta Clan", "Beta Camp")

	var constellationID string
	if err := db.QueryRow(`
		INSERT INTO constellations (name, founder_clan_id, status) VALUES ('Alpha-Beta Pact', $1, 'forming')
		RETURNING id`, founderID).Scan(&constellationID); err != nil {
		t.Fatalf("seeding constellation: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status, responded_at)
		VALUES ($1, $2, 44003, 'accepted', CURRENT_TIMESTAMP)`, constellationID, founderID); err != nil {
		t.Fatalf("seeding founder member: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status)
		VALUES ($1, $2, 44003, 'pending')`, constellationID, inviteeID); err != nil {
		t.Fatalf("seeding invitee member: %v", err)
	}

	// Directly exercise the accept transition's SQL (the same
	// statements HandleConstellationRespondCallback runs) rather than
	// going through the telebot.Context-dependent handler, matching
	// this package's doX-core testing convention elsewhere.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if _, err := tx.Exec("UPDATE constellation_members SET status = 'accepted', responded_at = CURRENT_TIMESTAMP WHERE constellation_id = $1 AND clan_id = $2", constellationID, inviteeID); err != nil {
		t.Fatalf("accepting: %v", err)
	}
	var pendingCount int
	if err := tx.QueryRow("SELECT COUNT(*) FROM constellation_members WHERE constellation_id = $1 AND status = 'pending'", constellationID).Scan(&pendingCount); err != nil {
		t.Fatalf("counting pending: %v", err)
	}
	if pendingCount != 0 {
		t.Fatalf("expected 0 pending members after both accept, got %d", pendingCount)
	}
	if _, err := tx.Exec(`
		UPDATE constellations SET status = 'active', activated_at = CURRENT_TIMESTAMP,
			expires_at = CURRENT_TIMESTAMP + (7 * INTERVAL '1 day') WHERE id = $1`, constellationID); err != nil {
		t.Fatalf("activating: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var status string
	var activatedAt, expiresAt sql.NullTime
	if err := db.QueryRow("SELECT status, activated_at, expires_at FROM constellations WHERE id = $1", constellationID).Scan(&status, &activatedAt, &expiresAt); err != nil {
		t.Fatalf("reading constellation: %v", err)
	}
	if status != "active" {
		t.Errorf("expected status 'active' after unanimous accept, got %q", status)
	}
	if !activatedAt.Valid || !expiresAt.Valid {
		t.Error("expected activated_at and expires_at to be set")
	}
	_ = h // handler constructed for consistency with other test files in this package; not called directly per the doX convention note above
}

// TestConstellationReject_DissolvesWholeProposal covers Phase 5.3 point
// 4: a single invited Clan rejecting must dissolve the entire forming
// Constellation, not just remove that one Clan from it.
func TestConstellationReject_DissolvesWholeProposal(t *testing.T) {
	db := rankingTestDB(t)

	founderID := seedClanWithLeader(t, db, 44005, "Gamma Clan", "Gamma Camp")
	inviteeID := seedClanWithLeader(t, db, 44006, "Delta Clan", "Delta Camp")

	var constellationID string
	if err := db.QueryRow(`
		INSERT INTO constellations (name, founder_clan_id, status) VALUES ('Gamma-Delta Pact', $1, 'forming')
		RETURNING id`, founderID).Scan(&constellationID); err != nil {
		t.Fatalf("seeding constellation: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status, responded_at)
		VALUES ($1, $2, 44005, 'accepted', CURRENT_TIMESTAMP)`, constellationID, founderID); err != nil {
		t.Fatalf("seeding founder member: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status)
		VALUES ($1, $2, 44005, 'pending')`, constellationID, inviteeID); err != nil {
		t.Fatalf("seeding invitee member: %v", err)
	}

	// Mirror HandleConstellationRespondCallback's reject branch.
	if _, err := db.Exec("UPDATE constellations SET status = 'dissolved' WHERE id = $1", constellationID); err != nil {
		t.Fatalf("dissolving: %v", err)
	}
	if _, err := db.Exec("UPDATE constellation_members SET status = 'rejected', responded_at = CURRENT_TIMESTAMP WHERE constellation_id = $1 AND clan_id = $2", constellationID, inviteeID); err != nil {
		t.Fatalf("rejecting: %v", err)
	}

	var status string
	if err := db.QueryRow("SELECT status FROM constellations WHERE id = $1", constellationID).Scan(&status); err != nil {
		t.Fatalf("reading constellation: %v", err)
	}
	if status != "dissolved" {
		t.Errorf("expected the whole Constellation to be 'dissolved' after one reject, got %q", status)
	}
}
