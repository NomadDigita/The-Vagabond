package handlers

import (
	"context"
	"testing"
)

// TestDoSetTaxRate_BroadcastsToAllRealPlayersNotAIFactions verifies the
// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.3 addition: a tax
// rate change now reaches every real player directly, not just the
// admin's own confirmation message, while AI factions (which have a
// synthetic users row but no Telegram session) are excluded.
func TestDoSetTaxRate_BroadcastsToAllRealPlayersNotAIFactions(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO tax_law (id, tax_rate_percent) VALUES (1, 5) ON CONFLICT (id) DO UPDATE SET tax_rate_percent = 5"); err != nil {
		t.Fatalf("seeding tax_law: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(9201)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO users (telegram_id, state) VALUES ($1, 'ai_faction')
		ON CONFLICT (telegram_id) DO UPDATE SET state = 'ai_faction'`, int64(-9202)); err != nil {
		t.Fatalf("seeding AI faction user: %v", err)
	}

	h := &AdminHandler{DB: db}
	if _, err := h.doSetTaxRate(ctx, 8); err != nil {
		t.Fatalf("doSetTaxRate: %v", err)
	}

	var realCount, aiCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", int64(9201)).Scan(&realCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1", int64(-9202)).Scan(&aiCount)

	if realCount == 0 {
		t.Error("expected the real player to receive a direct tax-rate-change notification")
	}
	if aiCount != 0 {
		t.Error("expected the AI faction's synthetic user row to receive no notification")
	}
}
