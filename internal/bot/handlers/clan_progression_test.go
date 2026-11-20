package handlers

import (
	"context"
	"database/sql"
	"testing"
)

func TestDoClanDonate_RecordsClanDonationProgressionEvent(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &ClanHandler{DB: db}

	leaderClanID := seedClanWithLeader(t, db, 46001, "Donor Clan", "Leader Camp")
	const memberID = 46002
	seedExchangeCamp(t, db, memberID, "Member Camp", 0, 0, 0)
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Soldier')", memberID, leaderClanID); err != nil {
		t.Fatalf("seeding member into clan: %v", err)
	}
	if _, err := db.Exec("UPDATE resources SET metal = 500 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $1)", memberID); err != nil {
		t.Fatalf("seeding donor metal: %v", err)
	}

	if _, err := h.doClanDonate(ctx, memberID, 46001, "metal", 100); err != nil {
		t.Fatalf("doClanDonate: unexpected error: %v", err)
	}

	var unlockedAt sql.NullTime
	if err := db.QueryRow("SELECT unlocked_at FROM player_achievements WHERE user_id = $1 AND achievement_key = 'generous_1'", memberID).Scan(&unlockedAt); err != nil {
		t.Fatalf("expected generous_1 achievement to be unlocked by a real clan donation: %v", err)
	}
	if !unlockedAt.Valid {
		t.Error("expected unlocked_at to be set")
	}
}
