package handlers

import "testing"

// TestIsRealPlayer_RejectsSyntheticAIFactionIDs guards the fix documented
// in BUGS_AND_INCONSISTENCIES.md: AI factions' seeded users rows use a
// synthetic negative telegram_id, and every "notify the other commander"
// site in combat_road_encounters.go must treat that as "nobody to notify,"
// not attempt a send that will fail and retry forever.
func TestIsRealPlayer_RejectsSyntheticAIFactionIDs(t *testing.T) {
	cases := []struct {
		userID int64
		want   bool
	}{
		{123456789, true},  // a normal Telegram ID
		{1, true},           // smallest positive - still real
		{0, false},          // zero: "no user" sentinel used throughout this codebase
		{-1, false},         // an AI faction's synthetic ID
		{-999999999, false}, // another AI faction's synthetic ID
	}
	for _, tc := range cases {
		if got := isRealPlayer(tc.userID); got != tc.want {
			t.Errorf("isRealPlayer(%d) = %v, want %v", tc.userID, got, tc.want)
		}
	}
}
