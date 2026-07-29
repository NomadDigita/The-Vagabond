package handlers

import (
	"context"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/roadcombat"
)

func seedGarrisonCamp(t *testing.T, telegramID int64, x, y int, isAIFaction bool) string {
	t.Helper()
	db := rankingTestDB(t)
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $2, 'plains', 'TestRegion', 'flat')
		RETURNING id`, x, y).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id, is_ai_faction) VALUES ($1, 'Garrison Test Camp', $2, $3) RETURNING id`,
		telegramID, coordID, isAIFaction).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO workshop_inventory (encampment_id) VALUES ($1)", campID); err != nil {
		t.Fatalf("seeding workshop_inventory: %v", err)
	}
	return campID
}

// TestLoadBaseGarrisonForce_AIFactionUsesSyntheticReserveFromLivePool
// verifies AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.4's fix:
// an AI faction's garrisoned_soldiers/garrisoned_mechs is never set by
// anyone, so reading it directly (the old behavior) always returned a
// free-win zero garrison. It should instead derive a reserve from the
// faction's live soldiers/mechs pool.
func TestLoadBaseGarrisonForce_AIFactionUsesSyntheticReserveFromLivePool(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedGarrisonCamp(t, 9501, 500, 500, true)

	if _, err := db.Exec("UPDATE workshop_inventory SET soldiers = 100, mechs = 50, garrisoned_soldiers = 0, garrisoned_mechs = 0 WHERE encampment_id = $1", camp); err != nil {
		t.Fatalf("seeding garrison: %v", err)
	}

	h := &CombatHandler{DB: db}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	force, err := h.loadBaseGarrisonForce(ctx, tx, camp, true)
	if err != nil {
		t.Fatalf("loadBaseGarrisonForce: %v", err)
	}
	if force.Soldiers != 20 || force.Mechs != 10 { // 20% of 100/50
		t.Errorf("expected a synthetic 20%% reserve (20 soldiers, 10 mechs), got %d soldiers, %d mechs", force.Soldiers, force.Mechs)
	}
}

// TestLoadBaseGarrisonForce_HumanReadsGarrisonedColumnUnchanged confirms
// the human path is untouched: it still reads the manual, explicit
// garrisoned_soldiers/garrisoned_mechs column, not the live pool.
func TestLoadBaseGarrisonForce_HumanReadsGarrisonedColumnUnchanged(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedGarrisonCamp(t, 9502, 501, 501, false)

	if _, err := db.Exec("UPDATE workshop_inventory SET soldiers = 100, mechs = 50, garrisoned_soldiers = 15, garrisoned_mechs = 5 WHERE encampment_id = $1", camp); err != nil {
		t.Fatalf("seeding garrison: %v", err)
	}

	h := &CombatHandler{DB: db}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	force, err := h.loadBaseGarrisonForce(ctx, tx, camp, false)
	if err != nil {
		t.Fatalf("loadBaseGarrisonForce: %v", err)
	}
	if force.Soldiers != 15 || force.Mechs != 5 {
		t.Errorf("expected the manually-set garrison (15, 5), got %d soldiers, %d mechs", force.Soldiers, force.Mechs)
	}
}

// TestApplyBaseGarrisonCasualties_AIFactionDebitsLivePoolNotWholeReserve
// verifies the write-back side: only the casualties are subtracted from
// the AI faction's live pool - the 80% that was never committed to this
// skirmish must be untouched.
func TestApplyBaseGarrisonCasualties_AIFactionDebitsLivePoolNotWholeReserve(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedGarrisonCamp(t, 9503, 502, 502, true)

	if _, err := db.Exec("UPDATE workshop_inventory SET soldiers = 100, mechs = 50 WHERE encampment_id = $1", camp); err != nil {
		t.Fatalf("seeding garrison: %v", err)
	}

	h := &CombatHandler{DB: db}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Of the synthetic 20-soldier/10-mech reserve, say 12 soldiers and 6
	// mechs were lost in the skirmish.
	lost := roadcombat.FieldForce{Soldiers: 12, Mechs: 6}
	if err := h.applyBaseGarrisonCasualties(ctx, tx, camp, lost, true); err != nil {
		t.Fatalf("applyBaseGarrisonCasualties: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var soldiers, mechs int
	if err := db.QueryRow("SELECT soldiers, mechs FROM workshop_inventory WHERE encampment_id = $1", camp).Scan(&soldiers, &mechs); err != nil {
		t.Fatalf("reading post-battle pool: %v", err)
	}
	if soldiers != 88 || mechs != 44 { // 100-12, 50-6 - NOT reset to some "survivors of 20/10" figure
		t.Errorf("expected only the casualties (12 soldiers, 6 mechs) debited from the live pool (100, 50), got (%d, %d)", soldiers, mechs)
	}
}

// TestApplyBaseGarrisonCasualties_HumanDebitsGarrisonedColumn confirms
// the human path still writes back to garrisoned_soldiers/garrisoned_mechs.
func TestApplyBaseGarrisonCasualties_HumanDebitsGarrisonedColumn(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	camp := seedGarrisonCamp(t, 9504, 503, 503, false)

	if _, err := db.Exec("UPDATE workshop_inventory SET garrisoned_soldiers = 15, garrisoned_mechs = 5 WHERE encampment_id = $1", camp); err != nil {
		t.Fatalf("seeding garrison: %v", err)
	}

	h := &CombatHandler{DB: db}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	lost := roadcombat.FieldForce{Soldiers: 5, Mechs: 2}
	if err := h.applyBaseGarrisonCasualties(ctx, tx, camp, lost, false); err != nil {
		t.Fatalf("applyBaseGarrisonCasualties: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var garrSoldiers, garrMechs int
	if err := db.QueryRow("SELECT garrisoned_soldiers, garrisoned_mechs FROM workshop_inventory WHERE encampment_id = $1", camp).Scan(&garrSoldiers, &garrMechs); err != nil {
		t.Fatalf("reading post-battle garrison: %v", err)
	}
	if garrSoldiers != 10 || garrMechs != 3 {
		t.Errorf("expected garrisoned_soldiers/mechs debited to (10, 3), got (%d, %d)", garrSoldiers, garrMechs)
	}
}
