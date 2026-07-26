package handlers

import "testing"

// TestGenerateReferralCode_Deterministic verifies that the same Telegram ID
// always produces the same code (needed so a code set at signup matches a
// code independently regenerated later, e.g. by /refer's fallback path).
func TestGenerateReferralCode_Deterministic(t *testing.T) {
	ids := []int64{1, 42, 123456789, 5123456789012, 999999999999999}
	for _, id := range ids {
		a := generateReferralCode(id)
		b := generateReferralCode(id)
		if a != b {
			t.Fatalf("generateReferralCode(%d) not deterministic: %q vs %q", id, a, b)
		}
	}
}

// TestGenerateReferralCode_Unique verifies distinct Telegram IDs never
// collide on the same code — the bug being fixed here is the old
// `telegramID % 1_000_000` scheme, which could map two different players to
// an identical code.
func TestGenerateReferralCode_Unique(t *testing.T) {
	ids := []int64{
		1, 2, 1000000, 1000001, 2000000,
		123456789, 1123456789, // differ by exactly 1_000_000,000 not 1_000_000 but still worth checking
		987654321, 987654322,
	}
	seen := make(map[string]int64)
	for _, id := range ids {
		code := generateReferralCode(id)
		if other, exists := seen[code]; exists && other != id {
			t.Fatalf("collision: telegram IDs %d and %d both produced code %q", other, id, code)
		}
		seen[code] = id
	}
}

// TestGenerateReferralCode_NoOldCollisionPattern specifically checks two IDs
// that WOULD have collided under the old `telegramID % 1_000_000` scheme no
// longer produce the same code.
func TestGenerateReferralCode_NoOldCollisionPattern(t *testing.T) {
	idA := int64(1123456)
	idB := idA + 1000000 // old scheme: idA % 1_000_000 == idB % 1_000_000

	codeA := generateReferralCode(idA)
	codeB := generateReferralCode(idB)

	if codeA == codeB {
		t.Fatalf("expected distinct codes for %d and %d, got %q for both", idA, idB, codeA)
	}
}

// TestGenerateReferralCode_HasPrefix ensures the REF prefix used throughout
// the UI (panel text, /start payload docs) is preserved.
func TestGenerateReferralCode_HasPrefix(t *testing.T) {
	code := generateReferralCode(42)
	if len(code) < 3 || code[:3] != "REF" {
		t.Fatalf("expected code to start with REF, got %q", code)
	}
}

// TestReferralMilestones_Ascending ensures milestone tiers are defined in
// strictly increasing order of Count, which the claiming loop in
// HandleFactionCallback relies on for correct "first unclaimed tier" logic.
func TestReferralMilestones_Ascending(t *testing.T) {
	for i := 1; i < len(referralMilestones); i++ {
		if referralMilestones[i].Count <= referralMilestones[i-1].Count {
			t.Fatalf("referralMilestones must be strictly ascending by Count, got %d then %d",
				referralMilestones[i-1].Count, referralMilestones[i].Count)
		}
	}
}
