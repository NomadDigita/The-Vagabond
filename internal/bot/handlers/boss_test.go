package handlers

import (
	"context"
	"testing"
)

// TestTierUnlocked_GatesHigherTiers is the direct test for
// NEXT_GENERATION_FEATURES_PLAN.md Phase 8's tier-gating rule: Tier 1
// is always unlocked; Tier 2 requires tier2KillRequirement combined
// Tier 1 kills server-wide; Tier 3 requires tier3KillRequirement
// combined Tier 2 kills.
func TestTierUnlocked_GatesHigherTiers(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	h := &BossHandler{DB: db}

	// Tier 1 is always unlocked, regardless of kill counts.
	if unlocked, _, _ := h.tierUnlocked(ctx, 1); !unlocked {
		t.Error("expected Tier 1 to always be unlocked")
	}

	// Fresh schema seed data has 0 kills on every boss - Tier 2/3
	// should both start locked.
	if unlocked, kills, required := h.tierUnlocked(ctx, 2); unlocked {
		t.Errorf("expected Tier 2 to start locked (kills=%d, required=%d)", kills, required)
	}
	if unlocked, kills, required := h.tierUnlocked(ctx, 3); unlocked {
		t.Errorf("expected Tier 3 to start locked (kills=%d, required=%d)", kills, required)
	}

	// Push a Tier 1 boss's total_kills up to the requirement.
	if _, err := db.Exec("UPDATE world_bosses SET total_kills = $1 WHERE tier = 1", tier2KillRequirement); err != nil {
		t.Fatalf("seeding tier 1 kills: %v", err)
	}
	if unlocked, kills, required := h.tierUnlocked(ctx, 2); !unlocked {
		t.Errorf("expected Tier 2 to unlock at %d/%d Tier 1 kills, got kills=%d required=%d", tier2KillRequirement, tier2KillRequirement, kills, required)
	}

	// Tier 3 should still be locked - it depends on Tier 2 kills, not
	// Tier 1.
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
