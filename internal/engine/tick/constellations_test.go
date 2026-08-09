package tick

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// seedActiveConstellation creates an 'active' Constellation with two
// member Clans (founder + one invited, both 'accepted'), each led by a
// distinct seeded encampment - returning the constellation id and both
// member encampment ids.
func seedActiveConstellation(t *testing.T, db *sql.DB, founderTelegramID, memberTelegramID int64, expiresInHours float64) (constellationID, founderCampID, memberCampID string) {
	t.Helper()

	founderCampID = seedEncampment(t, db, founderTelegramID, "Founder Camp", 40, 40, "TestRegion", false)
	memberCampID = seedEncampment(t, db, memberTelegramID, "Member Camp", 41, 41, "TestRegion", false)

	var founderClanID, memberClanID string
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ($1, $2) RETURNING id", "Founder Clan "+founderCampID[:8], founderTelegramID).Scan(&founderClanID); err != nil {
		t.Fatalf("seeding founder clan: %v", err)
	}
	if err := db.QueryRow("INSERT INTO clans (name, leader_id) VALUES ($1, $2) RETURNING id", "Member Clan "+memberCampID[:8], memberTelegramID).Scan(&memberClanID); err != nil {
		t.Fatalf("seeding member clan: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Leader')", founderTelegramID, founderClanID); err != nil {
		t.Fatalf("seeding founder user_clans: %v", err)
	}
	if _, err := db.Exec("INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Leader')", memberTelegramID, memberClanID); err != nil {
		t.Fatalf("seeding member user_clans: %v", err)
	}

	if err := db.QueryRow(`
		INSERT INTO constellations (name, banner_emoji, founder_clan_id, status, activated_at, expires_at)
		VALUES ('Test Constellation', '✨', $1, 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + ($2 * INTERVAL '1 hour'))
		RETURNING id`, founderClanID, expiresInHours).Scan(&constellationID); err != nil {
		t.Fatalf("seeding constellation: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status, responded_at)
		VALUES ($1, $2, $3, 'accepted', CURRENT_TIMESTAMP)`, constellationID, founderClanID, founderTelegramID); err != nil {
		t.Fatalf("seeding founder member: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status, responded_at)
		VALUES ($1, $2, $3, 'accepted', CURRENT_TIMESTAMP)`, constellationID, memberClanID, founderTelegramID); err != nil {
		t.Fatalf("seeding member member: %v", err)
	}
	return constellationID, founderCampID, memberCampID
}

// TestConstellationDividend_PaysEveryMemberClanOnce is the direct test
// for RARE_WORLD_FEATURES_PLAN.md Phase 5.4: an active Constellation
// due a dividend (last_dividend_at NULL) should pay
// constellationDividendDollars to every member Clan's encampments and
// set last_dividend_at.
func TestConstellationDividend_PaysEveryMemberClanOnce(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	constellationID, founderCampID, memberCampID := seedActiveConstellation(t, db, 54001, 54002, 168)

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.constellationDividend(ctx, tx) })

	var founderDollars, memberDollars float64
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", founderCampID).Scan(&founderDollars); err != nil {
		t.Fatalf("reading founder resources: %v", err)
	}
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", memberCampID).Scan(&memberDollars); err != nil {
		t.Fatalf("reading member resources: %v", err)
	}
	if founderDollars != constellationDividendDollars {
		t.Errorf("expected founder camp to receive %v, got %v", constellationDividendDollars, founderDollars)
	}
	if memberDollars != constellationDividendDollars {
		t.Errorf("expected member camp to receive %v, got %v", constellationDividendDollars, memberDollars)
	}

	var lastDividendAt sql.NullTime
	if err := db.QueryRow("SELECT last_dividend_at FROM constellations WHERE id = $1", constellationID).Scan(&lastDividendAt); err != nil {
		t.Fatalf("reading last_dividend_at: %v", err)
	}
	if !lastDividendAt.Valid {
		t.Error("expected last_dividend_at to be set after paying the dividend")
	}
}

// TestConstellationDividend_DoesNotPayTwiceWithin24Hours confirms the
// "once per day" throttle: running the phase again immediately after a
// dividend was just paid must not pay a second time.
func TestConstellationDividend_DoesNotPayTwiceWithin24Hours(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	_, founderCampID, _ := seedActiveConstellation(t, db, 54003, 54004, 168)

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.constellationDividend(ctx, tx) })
	runTickPhase(t, db, func(tx *sql.Tx) error { return e.constellationDividend(ctx, tx) })

	var founderDollars float64
	if err := db.QueryRow("SELECT dollars FROM resources WHERE encampment_id = $1", founderCampID).Scan(&founderDollars); err != nil {
		t.Fatalf("reading founder resources: %v", err)
	}
	if founderDollars != constellationDividendDollars {
		t.Errorf("expected exactly one dividend payment (%v), got %v after two passes", constellationDividendDollars, founderDollars)
	}
}

// TestConstellationExpire_DissolvesPastDeadlineAndLeavesLiveOnesAlone
// is the direct test for Phase 5.5: an active Constellation past its
// expires_at becomes 'dissolved', while one still within its window is
// untouched.
func TestConstellationExpire_DissolvesPastDeadlineAndLeavesLiveOnesAlone(t *testing.T) {
	db := testDB(t)
	e := NewEngine(db, time.Minute)
	ctx := context.Background()

	expiredID, _, _ := seedActiveConstellation(t, db, 54005, 54006, -1) // expired 1 hour ago
	liveID, _, _ := seedActiveConstellation(t, db, 54007, 54008, 168)   // 1 week out

	runTickPhase(t, db, func(tx *sql.Tx) error { return e.constellationExpire(ctx, tx) })

	var expiredStatus, liveStatus string
	if err := db.QueryRow("SELECT status FROM constellations WHERE id = $1", expiredID).Scan(&expiredStatus); err != nil {
		t.Fatalf("reading expired constellation: %v", err)
	}
	if err := db.QueryRow("SELECT status FROM constellations WHERE id = $1", liveID).Scan(&liveStatus); err != nil {
		t.Fatalf("reading live constellation: %v", err)
	}
	if expiredStatus != "dissolved" {
		t.Errorf("expected the past-deadline constellation to be dissolved, got %q", expiredStatus)
	}
	if liveStatus != "active" {
		t.Errorf("expected the still-active constellation to stay active, got %q", liveStatus)
	}
}
