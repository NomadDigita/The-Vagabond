package handlers

import (
	"context"
	"testing"
)

func TestTierUnlocked_GatesHigherTiers(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &BossHandler{DB: db}

	if unlocked, _, _ := h.tierUnlocked(ctx, 1); !unlocked {
		t.Error("expected Tier 1 to always be unlocked")
	}

	if unlocked, kills, required := h.tierUnlocked(ctx, 2); unlocked {
		t.Errorf("expected Tier 2 to start locked (kills=%d, required=%d)", kills, required)
	}
	if unlocked, kills, required := h.tierUnlocked(ctx, 3); unlocked {
		t.Errorf("expected Tier 3 to start locked (kills=%d, required=%d)", kills, required)
	}

	if _, err := db.Exec("UPDATE world_bosses SET total_kills = $1 WHERE tier = 1", tier2KillRequirement); err != nil {
		t.Fatalf("seeding tier 1 kills: %v", err)
	}
	if unlocked, kills, required := h.tierUnlocked(ctx, 2); !unlocked {
		t.Errorf("expected Tier 2 to unlock at %d/%d Tier 1 kills, got kills=%d required=%d", tier2KillRequirement, tier2KillRequirement, kills, required)
	}

	if unlocked, _, _ := h.tierUnlocked(ctx, 3); unlocked {
		t.Error("expected Tier 3 to remain locked - Tier 1 kills shouldn't unlock it")
	}

	if _, err := db.Exec("UPDATE world_bosses SET total_kills = $1 WHERE tier = 2", tier3KillRequirement); err != nil {
		t.Fatalf("seeding tier 2 kills: %v", err)
	}
	if unlocked, _, _ := h.tierUnlocked(ctx, 3); !unlocked {
		t.Error("expected Tier 3 to unlock once combined Tier 2 kills (including The Ravager, a Tier 2 Gauntlet boss) reach the requirement")
	}
}
